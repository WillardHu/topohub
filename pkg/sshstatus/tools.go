package sshstatus

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
)

// getSecretData retrieves username and password from Secret
func (c *sshStatusController) getSecretData(secretName, secretNamespace string) (string, string, string, bool, error) {
	c.log.Debugf("Attempting to get secret data for %s/%s", secretNamespace, secretName)

	c.log.Debugf("Fetching secret from Kubernetes API for %s/%s", secretNamespace, secretName)
	// Get authentication information from Secret
	secret, err := c.kubeClient.CoreV1().Secrets(secretNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		c.log.Errorf("Failed to get secret %s/%s: %v", secretNamespace, secretName, err)
		return "", "", "", false, err
	}

	username := string(secret.Data["username"])
	password := string(secret.Data["password"])
	sshKey := string(secret.Data["ssh-privatekey"])
	sshKeyAuth := sshKey != ""

	c.log.Debugf("Successfully retrieved secret data for %s/%s", secretNamespace, secretName)
	return username, password, sshKey, sshKeyAuth, nil
}

// compareSSHStatus checks if two SSHStatus are equal, ignoring pointer issues
func compareSSHStatus(a, b topohubv1beta1.SSHStatusStatus, logger *zap.SugaredLogger) bool {
	if a.Healthy != b.Healthy {
		if logger != nil {
			logger.Debugf("compareSSHStatus Healthy changed: %v -> %v", b.Healthy, a.Healthy)
		}
		return false
	}
	if a.LastUpdateTime != b.LastUpdateTime {
		if logger != nil {
			logger.Debugf("compareSSHStatus LastUpdateTime changed: %v -> %v", b.LastUpdateTime, a.LastUpdateTime)
		}
		return false
	}

	// Compare Basic fields
	if a.Basic.Type != b.Basic.Type {
		if logger != nil {
			logger.Debugf("compareSSHStatus Basic.Type changed: %v -> %v", b.Basic.Type, a.Basic.Type)
		}
		return false
	}
	if a.Basic.IpAddr != b.Basic.IpAddr {
		if logger != nil {
			logger.Debugf("compareSSHStatus Basic.IpAddr changed: %v -> %v", b.Basic.IpAddr, a.Basic.IpAddr)
		}
		return false
	}
	if a.Basic.Port != b.Basic.Port {
		if logger != nil {
			logger.Debugf("compareSSHStatus Basic.Port changed: %v -> %v", b.Basic.Port, a.Basic.Port)
		}
		return false
	}
	if a.Basic.SecretName != b.Basic.SecretName {
		if logger != nil {
			logger.Debugf("compareSSHStatus Basic.SecretName changed: %v -> %v", b.Basic.SecretName, a.Basic.SecretName)
		}
		return false
	}
	if a.Basic.SecretNamespace != b.Basic.SecretNamespace {
		if logger != nil {
			logger.Debugf("compareSSHStatus Basic.SecretNamespace changed: %v -> %v", b.Basic.SecretNamespace, a.Basic.SecretNamespace)
		}
		return false
	}
	if a.Basic.SSHKeyAuth != b.Basic.SSHKeyAuth {
		if logger != nil {
			logger.Debugf("compareSSHStatus Basic.SSHKeyAuth changed: %v -> %v", b.Basic.SSHKeyAuth, a.Basic.SSHKeyAuth)
		}
		return false
	}
	if a.Basic.ClusterName != b.Basic.ClusterName {
		if logger != nil {
			logger.Debugf("compareSSHStatus Basic.ClusterName changed: %v -> %v", b.Basic.ClusterName, a.Basic.ClusterName)
		}
		return false
	}

	// Compare Info fields
	if len(a.Info) != len(b.Info) {
		if logger != nil {
			logger.Debugf("compareSSHStatus Info length changed: %v -> %v", len(b.Info), len(a.Info))
		}
		return false
	}
	for k, v := range a.Info {
		if bv, ok := b.Info[k]; !ok || bv != v {
			if logger != nil {
				logger.Debugf("compareSSHStatus Info[%s] changed: %v -> %v", k, bv, v)
			}
			return false
		}
	}

	return true
}

// GenerateEvents creates Kubernetes events from SSH log entries and returns the latest messages and counts
func (c *sshStatusController) GenerateEvents(logEntries []map[string]string, sshStatusName string, lastLogTime string) (newLastestTime, newLastestMsg, newLastestWarningTime, newLastestWarningMsg string, totalMsgCount, warningMsgCount, newLogAccount int) {
	totalMsgCount = 0
	warningMsgCount = 0
	newLogAccount = 0
	newLastestTime = ""
	newLastestMsg = ""
	newLastestWarningTime = ""
	newLastestWarningMsg = ""

	if len(logEntries) == 0 {
		return
	}

	totalMsgCount = len(logEntries)
	for i, entry := range logEntries {
		timestamp := entry["timestamp"]
		level := entry["level"]
		message := entry["message"]

		msg := fmt.Sprintf("[%s][%s]: %s", timestamp, level, message)

		ty := corev1.EventTypeNormal
		if level == "ERROR" || level == "WARNING" {
			ty = corev1.EventTypeWarning
			if newLastestWarningMsg == "" {
				newLastestWarningTime = timestamp
				newLastestWarningMsg = msg
			}
			warningMsgCount++
		}

		// All new logs generate events
		if timestamp != lastLogTime {
			newLogAccount++
			c.log.Infof("find new log for sshStatus %s: %s", sshStatusName, msg)

			// Find the latest log
			if i == 0 {
				newLastestTime = timestamp
				newLastestMsg = msg
			}

			// Create event
			t := &corev1.ObjectReference{
				Kind:       topohubv1beta1.KindSSHStatus,
				Name:       sshStatusName,
				Namespace:  c.config.PodNamespace,
				APIVersion: topohubv1beta1.APIVersion,
			}
			c.recorder.Event(t, ty, "SSHLogEntry", msg)
		}
	}
	return
}
