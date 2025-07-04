package dhcpserver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
)

// startDnsmasq starts the dnsmasq process
func (s *dhcpServer) startDnsmasq() error {

	if err := s.setupInterface(); err != nil {
		return fmt.Errorf("failed to setup interface: %v", err)
	}

	err := s.generateDnsmasqConfig()
	if err != nil {
		return fmt.Errorf("failed to generate dnsmasq config: %v", err)
	}
	s.log.Infof("dnsmasq config file %s", s.configPath)

	// 创建 context 用于进程管理
	ctx, cancel := context.WithCancel(context.Background())
	s.cmdCancel = cancel

	// 启动 dnsmasq
	cmd := exec.Command("dnsmasq", "-C", s.configPath, "-d")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dnsmasq: %v", err)
	}

	s.cmd = cmd

	go func() {
		// cancel the ctx
		defer cancel()
		if err := cmd.Wait(); err != nil {
			if ctx.Err() == nil {
				s.log.Errorf("dnsmasq process exited unexpectedly: %v", err)
			}
		}
	}()

	// 等待进程启动
	time.Sleep(time.Second)

	// 检查进程是否正常运行
	if cmd.Process == nil {
		return fmt.Errorf("dnsmasq process failed to start")
	}

	// update the status of subnet
	s.statusUpdateCh <- struct{}{}

	return nil
}

// UpdateService updates the subnet configuration and restarts the DHCP server
func (s *dhcpServer) UpdateService(subnet topohubv1beta1.Subnet) error {
	s.lockData.Lock()
	// 更新 subnet
	s.subnet = &subnet
	s.lockData.Unlock()

	// 重启 DHCP 服务
	s.restartCh <- struct{}{}

	return nil
}

// monitor monitors the lease file and updates status
func (s *dhcpServer) monitor() {

	leaseWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		s.log.Errorf("Failed to create lease file watcher: %v", err)
		return
	}
	defer leaseWatcher.Close()
	if err := leaseWatcher.Add(filepath.Dir(s.leasePath)); err != nil {
		s.log.Errorf("Failed to watch lease file: %v", err)
		return
	}

	// watch the process at an interval
	tickerProcess := time.NewTicker(5 * time.Second)
	defer tickerProcess.Stop()

	s.lockData.RLock()
	subnetName := s.subnet.Name
	s.lockData.RUnlock()

	// 开始监控
	for {
		needRestart := false
		needReload := false
		needRenewConfig := false

		select {
		case <-s.stopCh:
			s.log.Errorf("subnet monitor is exiting")
			return

		// watch error of lease file
		case err := <-leaseWatcher.Errors:
			s.log.Errorf("Lease file watcher error: %v", err)

		// lease file event
		case event, ok := <-leaseWatcher.Events:
			if !ok {
				s.log.Panicf("Lease file watcher channel closed")
			}

			if event.Name == s.leasePath && (event.Op&fsnotify.Write == fsnotify.Write) {
				s.log.Infof("watcher lease file event: %+v", event)
				// inform new client to the redfishStatu
				if _, err := s.processDhcpLease(true); err != nil {
					s.log.Errorf("failed to processDhcpLease: %v", err)
					continue
				}
				// update the status of subnet
				s.statusUpdateCh <- struct{}{}
			} else {
				s.log.Debugf("watcher invalid file event: %+v", event)
			}

		case info := <-s.addedBindingIp:
			s.log.Debugf("process binding ip adding events for subnet %s: %+v", info.Subnet, info)
			//note: currently, it does not consider whether the ip is belonged to the ip range or not, which make it simple to handle the subnet changes
			if item, ok := s.currentManualBindingClients[info.IPAddr]; ok {
				if item.MAC != info.MacAddr {
					s.lockData.Lock()
					s.currentManualBindingClients[info.IPAddr] = &DhcpClientInfo{
						IP:       info.IPAddr,
						MAC:      info.MacAddr,
						Hostname: info.Hostname,
					}
					s.lockData.Unlock()
					s.log.Infof("update binding ip %s: old mac %s, new mac %s, hostname %s", info.IPAddr, item.MAC, info.MacAddr, info.Hostname)
				} else {
					continue
				}
			} else {
				s.lockData.Lock()
				s.currentManualBindingClients[info.IPAddr] = &DhcpClientInfo{
					IP:       info.IPAddr,
					MAC:      info.MacAddr,
					Hostname: info.Hostname,
				}
				s.lockData.Unlock()
				s.log.Infof("add new binding ip %s: %+v", info.IPAddr, info)
			}
			if err := s.UpdateDhcpBindings(); err != nil {
				s.log.Errorf("failed to add dhcp bindings: %v", err)
				continue
			}
			// it has been renew the config
			needRenewConfig = false
			needReload = true

		case info := <-s.deletedBindingIp:
			s.log.Debugf("process binding ip deleting events for subnet %s: %+v", info.Subnet, info)
			//note: currently, it does not consider whether the ip is belonged to the ip range or not, which make it simple to handle the subnet changes
			if item, ok := s.currentManualBindingClients[info.IPAddr]; ok && strings.EqualFold(item.MAC, info.MacAddr) {
				delete(s.currentManualBindingClients, info.IPAddr)
				s.log.Infof("delete binding ip %s: %+v", info.IPAddr, info)
			} else {
				continue
			}
			if err := s.UpdateDhcpBindings(); err != nil {
				s.log.Errorf("failed to delete dhcp bindings: %v", err)
				continue
			}
			// it has been renew the config
			needRenewConfig = false
			needReload = true

		// reconcile notify subnet spec changes
		case <-s.restartCh:
			needRenewConfig = true
			needReload = true
			s.log.Infof("dhcp server reload after the spec of subnet is updated")

		// check the process
		case <-tickerProcess.C:
			isDead := s.cmd == nil || s.cmd.Process == nil
			if !isDead {
				if err := s.cmd.Process.Signal(syscall.Signal(0)); err != nil {
					s.log.Errorf("DHCP server process check failed: %v", err)
					needRenewConfig = true
					needRestart = true
				} else {
					s.log.Debugf("dhcp server for %s is running", subnetName)
				}
			} else {
				needRenewConfig = true
				needRestart = true
				s.log.Infof("dhcp server for %s is dead, restart it", subnetName)
			}
		}

		if needRenewConfig {
			if err := s.generateDnsmasqConfig(); err != nil {
				s.log.Errorf("Failed to update dnsmasq config: %v", err)
				continue
			}
		}

		if needReload {
			s.log.Infof("reload dhcp server")
			// 重新加载 dnsmasq 配置
			if err := s.cmd.Process.Signal(syscall.SIGHUP); err != nil {
				s.log.Errorf("failed to reload dnsmasq: %v", err)
				continue
			}
			s.log.Infof("Reloaded dnsmasq config: %s", s.configPath)
			// update the status of subnet
			s.statusUpdateCh <- struct{}{}

		} else if needRestart {
			s.log.Infof("restarting dhcp server")
			// in the startDnsmasq, it finish 's.statusUpdateCh <- struct{}{}'
			if err := s.startDnsmasq(); err != nil {
				s.log.Errorf("Failed to restart dnsmasq: %v", err)
			}
			// in the startDnsmasq, it finish 's.statusUpdateCh <- struct{}{}'
			// s.statusUpdateCh <- struct{}{}
		}

	}
}
