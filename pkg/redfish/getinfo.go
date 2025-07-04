package redfish

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/stmcginnis/gofish/redfish"
)

func setData(result map[string]string, key, value string) {
	if len(value) == 0 {
		result[key] = ""
	} else {
		result[key] = value
	}
}

const (
	DeviceType_Unknown = "Unknown"
	DeviceType_GPU     = "GPU"
	DeviceType_Storage = "STORAGE"
	DeviceType_NIC     = "NIC"
)

func (c *redfishClient) GetInfo() (map[string]string, error) {

	result := map[string]string{}

	// Attached the client to service root
	service := c.client.Service

	// Query the managers for bmc
	managers, err := service.Managers()
	if err != nil {
		c.logger.Errorf("failed to Query the bmc : %+v", err)
		return nil, err
	} else if len(managers) == 0 {
		c.logger.Errorf("failed to get bmc")
		return nil, fmt.Errorf("failed to get bmc")
	}
	c.logger.Debugf("bmc amount: %d", len(managers))
	// for n, t := range managers {
	// 	c.logger.Debugf("bmc[%d]: %+v", n, *t)
	// }

	bmc := managers[0]
	// bmc info
	setData(result, "BmcFirmwareVersion", bmc.FirmwareVersion)
	setData(result, "BmcStatus", string(bmc.Status.Health))

	// Query the computer systems
	ss, err := service.Systems()
	if err != nil {
		c.logger.Errorf("failed to Query the computer systems: %+v", err)
		return nil, err
	} else if len(ss) == 0 {
		c.logger.Errorf("failed to get system")
		return nil, fmt.Errorf("failed to get system")
	}
	c.logger.Debugf("system amount: %d", len(ss))
	// for n, t := range ss {
	// 	c.logger.Debugf("systems[%d]: %+v", n, *t)
	// }

	// for barel metal case,
	system := ss[0]
	// basic info
	setData(result, "BiosVerison", system.BIOSVersion)
	setData(result, "HostName", system.HostName)
	setData(result, "Manufacturer", system.Manufacturer)
	setData(result, "PowerState", string(system.PowerState))
	setData(result, "SystemStatus", string(system.Status.Health))
	setData(result, "RedfishVersion", service.RedfishVersion)
	setData(result, "Vendor", service.Vendor)
	// get supported reset types
	joinStr := c.GetSupportedResetTypes(system)
	setData(result, "SupportedReset", joinStr)

	// cpu info
	setData(result, "CpuPhysicalCore", fmt.Sprintf("%d", system.ProcessorSummary.Count))
	setData(result, "CpuLogicalCore", fmt.Sprintf("%d", system.ProcessorSummary.LogicalProcessorCount))
	setData(result, "CpuModel", system.ProcessorSummary.Model)
	setData(result, "CpuStatus", string(system.ProcessorSummary.Status.Health))
	cpus, err := system.Processors()
	if err != nil {
		c.logger.Errorf("failed to get processors: %+v", err)
		return nil, err
	}
	c.logger.Debugf("cpus amount: %d", len(cpus))
	for n, cpu := range cpus {
		//c.logger.Debugf("Cpu[%d]: %+v", n, cpu)
		setData(result, fmt.Sprintf("Cpu[%d].Manufacturer", n), string(cpu.Manufacturer))
		setData(result, fmt.Sprintf("Cpu[%d].ProcessorType", n), string(cpu.ProcessorType))
		setData(result, fmt.Sprintf("Cpu[%d].Health", n), string(cpu.Status.Health))
		setData(result, fmt.Sprintf("Cpu[%d].State", n), string(cpu.Status.State))
		// theses fields is dynamic, so we don't set them
		//setData(result, fmt.Sprintf("Cpu[%d].TotalCores", n), fmt.Sprintf("%d", cpu.TotalCores))
		//setData(result, fmt.Sprintf("Cpu[%d].TotalThreads", n), fmt.Sprintf("%d", cpu.TotalThreads))
		//setData(result, fmt.Sprintf("Cpu[%d].MaxSpeedMHz", n), fmt.Sprintf("%.2f", float64(cpu.MaxSpeedMHz)/1000))
		//setData(result, fmt.Sprintf("Cpu[%d].Architecture", n), string(cpu.ProcessorArchitecture))
		//setData(result, fmt.Sprintf("Cpu[%d].Model", n), cpu.Model)
	}

	// memory info
	setData(result, "MemoryTotalGiB", fmt.Sprintf("%.0f", system.MemorySummary.TotalSystemMemoryGiB))
	setData(result, "MemoryStatus", string(system.MemorySummary.Status.Health))
	mms, err := system.Memory()
	if err != nil {
		c.logger.Errorf("failed to get memory: %+v", err)
		return nil, err
	}
	setData(result, "MemoryChipsAccount", fmt.Sprintf("%d", len(mms)))
	//在内存条不变时，有时数组的顺序的变换，导致 后续 redfishstatus 会做无意义的更新，暂时 取消这些信息
	for n, mm := range mms {
		//c.logger.Debugf("Memory[%d]: %+v", n, mm)
		setData(result, fmt.Sprintf("Memory[%d].Manufacturer", n), string(mm.Manufacturer))
		setData(result, fmt.Sprintf("Memory[%d].MemoryType", n), string(mm.MemoryType))
		setData(result, fmt.Sprintf("Memory[%d].MemoryDeviceType", n), string(mm.MemoryDeviceType))
		setData(result, fmt.Sprintf("Memory[%d].Manufacturer", n), string(mm.Manufacturer))
		setData(result, fmt.Sprintf("Memory[%d].Model", n), string(mm.Model))
		setData(result, fmt.Sprintf("Memory[%d].CapacityGiB", n), fmt.Sprintf("%.2f", float64(mm.CapacityMiB)/1024))
		setData(result, fmt.Sprintf("Memory[%d].Health", n), string(mm.Status.Health))
		setData(result, fmt.Sprintf("Memory[%d].State", n), string(mm.Status.State))
		// theses fields is dynamic, so we don't set them
		//setData(result, fmt.Sprintf("Memory[%d].Name", n), string(mm.Name))
		//if len(mm.AllowedSpeedsMHz) > 0 {
		//	setData(result, fmt.Sprintf("Memory[%d].AllowedSpeedsMHz", n), fmt.Sprintf("%d", mm.AllowedSpeedsMHz[0]))
		//}
		//setData(result, fmt.Sprintf("Memory[%d].OperatingSpeedMhz", n), fmt.Sprintf("%d", mm.OperatingSpeedMhz))
	}

	// storage info
	storages, err := system.Storage()
	if err != nil {
		c.logger.Errorf("failed to get storage: %+v", err)
		return nil, err
	}
	c.logger.Debugf("storage amount: %d", len(storages))
	for n, st := range storages {
		c.logger.Debugf("Storage[%d]: %+v", n, st)
		setData(result, fmt.Sprintf("Storage[%d].Name", n), st.Name)
		setData(result, fmt.Sprintf("Storage[%d].Id", n), st.ID)
		setData(result, fmt.Sprintf("Storage[%d].Health", n), string(st.Status.Health))
		setData(result, fmt.Sprintf("Storage[%d].State", n), string(st.Status.State))

		// 获取驱动器信息
		drives, err := st.Drives()
		if err != nil {
			c.logger.Errorf("failed to get drives for storage[%d]: %+v", n, err)
			continue
		}

		c.logger.Debugf("Storage[%d] drives amount: %d", n, len(drives))
		for m, drive := range drives {
			c.logger.Debugf("Storage[%d].Drive[%d]: %+v", n, m, drive)
			setData(result, fmt.Sprintf("Storage[%d].Drive[%d].Name", n, m), drive.Name)
			setData(result, fmt.Sprintf("Storage[%d].Drive[%d].Model", n, m), drive.Model)
			setData(result, fmt.Sprintf("Storage[%d].Drive[%d].Manufacturer", n, m), drive.Manufacturer)
			setData(result, fmt.Sprintf("Storage[%d].Drive[%d].SerialNumber", n, m), drive.SerialNumber)
			setData(result, fmt.Sprintf("Storage[%d].Drive[%d].MediaType", n, m), string(drive.MediaType))
			setData(result, fmt.Sprintf("Storage[%d].Drive[%d].Protocol", n, m), string(drive.Protocol))
			setData(result, fmt.Sprintf("Storage[%d].Drive[%d].CapacityGiB", n, m), fmt.Sprintf("%.2f", float64(drive.CapacityBytes)/(1024*1024*1024)))
			setData(result, fmt.Sprintf("Storage[%d].Drive[%d].Health", n, m), string(drive.Status.Health))
			setData(result, fmt.Sprintf("Storage[%d].Drive[%d].State", n, m), string(drive.Status.State))
		}
	}

	// network info
	// interfaces, err := system.NetworkInterfaces()
	// if err != nil {
	// 	c.logger.Errorf("failed to get network interfaces: %+v", err)
	// 	return nil, err
	// }
	// for n, item := range interfaces {
	// 	c.logger.Debugf("NetworkInterfaces[%d]: %+v", n, item.NetworkAdapter())

	// 	adapter, err := item.NetworkAdapter()
	// 	if err != nil {
	// 		c.logger.Errorf("failed to get network adapter: %+v", err)
	// 		return nil, err
	// 	}
	// 	setData(result, fmt.Sprintf("NetworkAdapter[%d].Manufacturer", n), adapter.Manufacturer)
	// 	setData(result, fmt.Sprintf("NetworkAdapter[%d].Model", n), adapter.Model)
	// 	setData(result, fmt.Sprintf("NetworkAdapter[%d].Name", n), adapter.Name)
	// 	for m, c := range adapter.Controllers {
	// 		setData(result, fmt.Sprintf("NetworkAdapter[%d].Controllers[%d].Manufacturer", n, m), c.FirmwarePackageVersion)
	// 		setData(result, fmt.Sprintf("NetworkAdapter[%d].Controllers[%d].PCIeType", n, m), string(c.PCIeInterface.PCIeType))
	// 		setData(result, fmt.Sprintf("NetworkAdapter[%d].Controllers[%d].MaxPCIeType", n, m), string(c.PCIeInterface.MaxPCIeType))
	// 		setData(result, fmt.Sprintf("NetworkAdapter[%d].Controllers[%d].LanesInUse", n, m), string(c.PCIeInterface.LanesInUse))
	// 		setData(result, fmt.Sprintf("NetworkAdapter[%d].Controllers[%d].MaxLanes", n, m), string(c.PCIeInterface.MaxLanes))
	// 		setData(result, fmt.Sprintf("NetworkAdapter[%d].Controllers[%d].NetworkPortCount", n, m), string(c.ControllerCapabilities.NetworkPortCount))
	// 	}
	// }

	// pcie info
	cs, err := service.Chassis()
	if err != nil {
		c.logger.Errorf("failed to get chassis: %+v", err)
		if len(cs) == 0 {
			return nil, fmt.Errorf("failed to get chassis")
		}
	}

	c.logger.Debugf("chassis amount: %d", len(cs))
	for count, chassis := range cs {
		pcieList, err := chassis.PCIeDevices()
		if err != nil {
			c.logger.Errorf("failed to get pcie devices: %+v", err)
			return nil, err
		}
		c.logger.Debugf("chassis[%d] pcie devices amount: %d", count, len(pcieList))
		if len(pcieList) == 0 {
			continue
		}

	LOOP_PCIEDEVICE:
		for m, item := range pcieList {
			// c.logger.Debugf("PCIeDevices[%d]: %+v", m, item)

			switch strings.ToLower(item.Description) {
			case "GPU Device":
				setData(result, fmt.Sprintf("PCIeDevices[%d].DeviceType", m), DeviceType_GPU)
			case "NVMeSSD Device":
				setData(result, fmt.Sprintf("PCIeDevices[%d].DeviceType", m), DeviceType_Storage)
			case "NIC device":
				setData(result, fmt.Sprintf("PCIeDevices[%d].DeviceType", m), DeviceType_NIC)
			default:
				setData(result, fmt.Sprintf("PCIeDevices[%d].DeviceType", m), DeviceType_Unknown)
			}

			setData(result, fmt.Sprintf("PCIeDevices[%d].Name", m), item.Name)
			setData(result, fmt.Sprintf("PCIeDevices[%d].Manufacturer", m), item.Manufacturer)
			setData(result, fmt.Sprintf("PCIeDevices[%d].Model", m), item.Model)
			setData(result, fmt.Sprintf("PCIeDevices[%d].Description", m), item.Description)
			setData(result, fmt.Sprintf("PCIeDevices[%d].FirmwareVersion", m), item.FirmwareVersion)
			setData(result, fmt.Sprintf("PCIeDevices[%d].PCIeType", m), string(item.PCIeInterface.PCIeType))
			setData(result, fmt.Sprintf("PCIeDevices[%d].MaxPCIeType", m), string(item.PCIeInterface.MaxPCIeType))
			setData(result, fmt.Sprintf("PCIeDevices[%d].LanesInUse", m), fmt.Sprintf("%d", item.PCIeInterface.LanesInUse))
			setData(result, fmt.Sprintf("PCIeDevices[%d].MaxLanes", m), fmt.Sprintf("%d", item.PCIeInterface.MaxLanes))
			setData(result, fmt.Sprintf("PCIeDevices[%d].Health", m), string(item.Status.Health))
			setData(result, fmt.Sprintf("PCIeDevices[%d].State", m), string(item.Status.State))

			pfcs, err := item.PCIeFunctions()
			if err == nil && len(pfcs) > 0 {
				c.logger.Debugf("pcie devices[%d] functions amount: %d", m, len(pfcs))
				for n, pfc := range pfcs {
					c.logger.Debugf("PCIeDevices[%d].PCIeFunctions[%d]: %+v", m, n, pfc)
					// for network device function
					ints, err := pfc.EthernetInterfaces()
					if err == nil && len(ints) > 0 {
						setData(result, fmt.Sprintf("PCIeDevices[%d].NetworkInterfacePortCount", m), fmt.Sprintf("%d", len(ints)))
						for t, netint := range ints {
							c.logger.Debugf("PCIeDevices[%d].PCIeFunctions[%d].EthernetInterfaces[%d]: %+v", m, n, t, netint)
							setData(result, fmt.Sprintf("PCIeDevices[%d].Functions[%d].EthernetInterfaces[%d].MACAddress", m, n, t), netint.MACAddress)
							setData(result, fmt.Sprintf("PCIeDevices[%d].Functions[%d].EthernetInterfaces[%d].SpeedGbps", m, n, t), fmt.Sprintf("%.2f", float64(netint.SpeedMbps)/1000))
							setData(result, fmt.Sprintf("PCIeDevices[%d].Functions[%d].EthernetInterfaces[%d].State", m, n, t), string(netint.Status.State))
							setData(result, fmt.Sprintf("PCIeDevices[%d].Functions[%d].EthernetInterfaces[%d].Health", m, n, t), string(netint.Status.Health))
						}
						continue LOOP_PCIEDEVICE
					}

					// for storage device function
					stors, err := pfc.StorageControllers()
					if err == nil && len(stors) > 0 {
						setData(result, fmt.Sprintf("PCIeDevices[%d].StorageControllerPortCount", m), fmt.Sprintf("%d", len(stors)))
						for t, stor := range stors {
							c.logger.Debugf("PCIeDevices[%d].Functions[%d].StorageControllers[%d]: %+v", m, n, t, stor)
							setData(result, fmt.Sprintf("PCIeDevices[%d].Functions[%d].StorageControllers[%d].Health", m, n, t), string(stor.Status.Health))
							setData(result, fmt.Sprintf("PCIeDevices[%d].Functions[%d].StorageControllers[%d].State", m, n, t), string(stor.Status.State))
						}
						continue LOOP_PCIEDEVICE
					}
				}
			}

		}

		break
	}

	// ?? 是否可以取出安装的 os 信息

	return result, nil
}

type ResetActionInfo struct {
	Description string `json:"Description"`
	Id          string `json:"Id"`
	Name        string `json:"Name"`
	Parameters  []struct {
		AllowableValues []string `json:"AllowableValues"`
		DataType        string   `json:"DataType"`
		Name            string   `json:"Name"`
		Required        bool     `json:"Required"`
	} `json:"Parameters"`
}

// GetSupportedResetTypes 获取支持的ResetTypes类型
func (c *redfishClient) GetSupportedResetTypes(system *redfish.ComputerSystem) string {
	joinStr := ""

	c.logger.Debug("Get reset types from system SupportedResetTypes", "SupportedResetTypes", system.SupportedResetTypes)
	for _, item := range system.SupportedResetTypes {
		if len(joinStr) > 0 {
			joinStr += ","
		}
		joinStr += string(item)
	}
	if joinStr != "" {
		return joinStr
	}

	// 尝试从ResetActionInfo获取
	resetInfoPath := fmt.Sprintf("/redfish/v1/Systems/%s/ResetActionInfo", system.ID)
	c.logger.Debugf("Trying to get reset types from ResetActionInfo URL: %s", resetInfoPath)
	resp, err := c.client.Get(resetInfoPath)
	if err != nil {
		c.logger.Errorf("Failed to get ResetActionInfo: %v", err)
		return ""
	}
	defer resp.Body.Close()

	// 读取并打印原始响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Errorf("Failed to get ResetActionInfo: %v", err)
		return ""
	}
	c.logger.Debugf("ResetActionInfo response: %s", string(respBody))

	// 解析JSON
	var actionInfo ResetActionInfo
	err = json.Unmarshal(respBody, &actionInfo)
	if err != nil {
		c.logger.Errorf("Failed to decode ResetActionInfo: %v", err)
		return ""
	}

	// 查找ResetType参数
	for _, param := range actionInfo.Parameters {
		if param.Name == "ResetType" {
			c.logger.Debugf("Found ResetType parameter with values: %v", param.AllowableValues)
			for _, value := range param.AllowableValues {
				if len(joinStr) > 0 {
					joinStr += ","
				}
				joinStr += value
			}
			return joinStr
		}
	}

	return ""
}
