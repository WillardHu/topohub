package config

import (
	"fmt"
	"os"
	"path/filepath"

	"os/exec"

	"github.com/infrastructure-io/topohub/pkg/log"
	"github.com/infrastructure-io/topohub/pkg/tools"
	"gopkg.in/yaml.v2"
)

// AgentConfig represents the agent configuration
type AgentConfig struct {

	// pod namespace
	PodNamespace string

	// node name
	NodeName string

	// webhook cert dir
	WebhookCertDir string

	// storage path
	StoragePath                         string
	StoragePathDhcpLog                  string
	StoragePathDhcpLease                string
	StoragePathDhcpConfig               string
	StoragePathHttp                     string
	StoragePathHttpZtp                  string
	StoragePathHttpIso                  string
	StoragePathHttpTools                string
	StoragePathTftp                     string
	StoragePathTftpRelativeDirForPxeEfi string
	StoragePathTftpAbsoluteDirForPxeEfi string

	// dnsmasq config template path
	DhcpConfigTemplatePath string

	// FeatureConfigPath is the path to the feature configuration file
	FeatureConfigPath string
	// Redfish configuration
	RedfishPort                 int
	RedfishHttps                bool
	RedfishSecretName           string
	RedfishSecretNamespace      string
	RedfishStatusUpdateInterval int
	SSHStatusUpdateInterval     int
	// DHCP server configuration
	DhcpServerInterface string
	HttpEnabled         bool
	HttpPort            string
}

// FeatureConfig represents the feature configuration loaded from YAML
type FeatureConfig struct {
	RedfishPort                 int    `yaml:"redfishPort"`
	RedfishHttps                bool   `yaml:"redfishHttps"`
	RedfishSecretname           string `yaml:"redfishSecretname"`
	RedfishSecretNamespace      string `yaml:"redfishSecretNamespace"`
	RedfishStatusUpdateInterval int    `yaml:"redfishStatusUpdateInterval"`
	SSHStatusUpdateInterval     int    `yaml:"sshStatusUpdateInterval"`
	DhcpServerInterface         string `yaml:"dhcpServerInterface"`
	HttpServerPort              string `yaml:"httpServerPort"`
	HttpServerEnabled           bool   `yaml:"httpServerEnabled"`
}

// LoadFeatureConfig loads feature configuration from the config file
func (c *AgentConfig) loadFeatureConfig() error {
	// Read the feature-config.yaml file
	configPath := filepath.Join(c.FeatureConfigPath, "feature-config.yaml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read feature-config.yaml: %v", err)
	}

	// Parse YAML configuration
	var featureConfig FeatureConfig
	if err := yaml.Unmarshal(configData, &featureConfig); err != nil {
		return fmt.Errorf("failed to parse feature-config.yaml: %v", err)
	}

	c.RedfishPort = featureConfig.RedfishPort
	c.RedfishHttps = featureConfig.RedfishHttps
	c.RedfishSecretName = featureConfig.RedfishSecretname
	c.RedfishSecretNamespace = featureConfig.RedfishSecretNamespace
	c.RedfishStatusUpdateInterval = featureConfig.RedfishStatusUpdateInterval
	c.SSHStatusUpdateInterval = featureConfig.SSHStatusUpdateInterval
	c.DhcpServerInterface = featureConfig.DhcpServerInterface
	c.HttpPort = featureConfig.HttpServerPort
	c.HttpEnabled = featureConfig.HttpServerEnabled

	// 验证必要的字段
	if len(c.DhcpServerInterface) == 0 {
		return fmt.Errorf("dhcpServerInterface is empty")
	}

	// 验证接口是否存在
	if err := tools.ValidateInterfaceExists(c.DhcpServerInterface); err != nil {
		return fmt.Errorf("failed to find dhcpServer Interface %s: %v", c.DhcpServerInterface, err)
	}

	return nil
}

// verifyWebhookCertDir verifies that the webhook certificate directory exists and contains required files
func (c *AgentConfig) verifyWebhookCertDir() error {
	requiredFiles := []string{"tls.crt", "tls.key", "ca.crt"}
	for _, file := range requiredFiles {
		path := filepath.Join(c.WebhookCertDir, file)
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("required webhook certificate file %s not found: %v", file, err)
		}
	}
	return nil
}

func (c *AgentConfig) initStorageDirectory() error {
	// Check if main storage directory exists
	if _, err := os.Stat(c.StoragePath); err != nil {
		return fmt.Errorf("did not exist storage path %s: %v", c.StoragePath, err)
	}

	c.StoragePathDhcpLease = filepath.Join(c.StoragePath, "dhcp/lease")
	c.StoragePathDhcpConfig = filepath.Join(c.StoragePath, "dhcp/config")
	c.StoragePathDhcpLog = filepath.Join(c.StoragePath, "dhcp/log")
	c.StoragePathTftp = filepath.Join(c.StoragePath, "tftp")
	c.StoragePathTftpRelativeDirForPxeEfi = "boot/grub/x86_64-efi"
	c.StoragePathTftpAbsoluteDirForPxeEfi = filepath.Join(c.StoragePathTftp, c.StoragePathTftpRelativeDirForPxeEfi)
	c.StoragePathHttp = filepath.Join(c.StoragePath, "http")
	c.StoragePathHttpZtp = filepath.Join(c.StoragePathHttp, "ztp")
	c.StoragePathHttpIso = filepath.Join(c.StoragePathHttp, "iso")
	c.StoragePathHttpTools = filepath.Join(c.StoragePathHttp, "tools")

	// List of required subdirectories
	subdirs := []string{
		c.StoragePathDhcpLease,
		c.StoragePathDhcpConfig,
		c.StoragePathDhcpLog,
		c.StoragePathTftp,
		c.StoragePathTftpAbsoluteDirForPxeEfi,
		c.StoragePathHttp,
		c.StoragePathHttpIso,
		c.StoragePathHttpZtp,
	}

	// Check and create each subdirectory if it doesn't exist
	for _, dir := range subdirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create subdirectory %s: %v", dir, err)
			}
		}
	}

	// Set ownership and permissions for TFTP directory
	if err := os.Chown(c.StoragePathTftp, 65534, 65534); err != nil { // 65534 is nobody:nogroup
		return fmt.Errorf("failed to change ownership of TFTP directory: %v", err)
	}
	if err := os.Chmod(c.StoragePathTftp, 0777); err != nil {
		return fmt.Errorf("failed to change permissions of TFTP directory: %v", err)
	}

	// Copy core.efi file if it exists
	sourceFile := "/files/core.efi"
	targetFile := filepath.Join(c.StoragePathTftpAbsoluteDirForPxeEfi, "core.efi")
	if _, err := os.Stat(targetFile); err != nil && os.IsNotExist(err) {
		if _, err := os.Stat(sourceFile); err == nil {
			log.Logger.Infof("%s exists, copying to %s", sourceFile, targetFile)
			input, err := os.ReadFile(sourceFile)
			if err != nil {
				return fmt.Errorf("failed to read core.efi: %v", err)
			}
			if err := os.WriteFile(targetFile, input, 0644); err != nil {
				return fmt.Errorf("failed to copy core.efi to %s: %v", targetFile, err)
			}
			log.Logger.Infof("Successfully copied core.efi to %s", targetFile)
		} else {
			return fmt.Errorf("source core.efi not found")
		}
	}

	// Delete and recreate tools directory
	if err := os.RemoveAll(c.StoragePathHttpTools); err != nil {
		return fmt.Errorf("failed to delete tools directory %s: %v", c.StoragePathHttpTools, err)
	}
	if err := os.MkdirAll(c.StoragePathHttpTools, 0755); err != nil {
		return fmt.Errorf("failed to create tools directory %s: %v", c.StoragePathHttpTools, err)
	}

	// Copy all files from /tools to c.StoragePathHttpTools
	cmd := exec.Command("cp", "-r", "/tools/.", c.StoragePathHttpTools)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy tools: %v, output: %s", err, string(output))
	}

	return nil
}

func LoadAgentConfig() (*AgentConfig, error) {
	agentConfig := &AgentConfig{}

	// Load environment variables
	agentConfig.PodNamespace = os.Getenv("POD_NAMESPACE")
	if agentConfig.PodNamespace == "" {
		return nil, fmt.Errorf("POD_NAMESPACE environment variable not set")
	}

	agentConfig.NodeName = os.Getenv("NODE_NAME")
	if agentConfig.NodeName == "" {
		return nil, fmt.Errorf("NODE_NAME environment variable not set")
	}

	agentConfig.WebhookCertDir = os.Getenv("WEBHOOK_CERT_DIR")
	if agentConfig.WebhookCertDir == "" {
		return nil, fmt.Errorf("WEBHOOK_CERT_DIR environment variable not set")
	}

	agentConfig.StoragePath = os.Getenv("STORAGE_PATH")
	if agentConfig.StoragePath == "" {
		return nil, fmt.Errorf("STORAGE_PATH environment variable not set")
	}

	agentConfig.FeatureConfigPath = os.Getenv("FEATURE_CONFIG_PATH")
	if agentConfig.FeatureConfigPath == "" {
		return nil, fmt.Errorf("FEATURE_CONFIG_PATH environment variable not set")
	}

	agentConfig.DhcpConfigTemplatePath = os.Getenv("DHCP_CONFIG_TEMPLATE_PATH")
	if agentConfig.DhcpConfigTemplatePath == "" {
		return nil, fmt.Errorf("DHCP_CONFIG_TEMPLATE_PATH environment variable not set")
	}

	// Load feature configuration
	if err := agentConfig.loadFeatureConfig(); err != nil {
		return nil, fmt.Errorf("failed to load feature configuration: %v", err)
	}

	// Verify webhook certificate directory
	if err := agentConfig.verifyWebhookCertDir(); err != nil {
		return nil, fmt.Errorf("webhook certificate verification failed: %v", err)
	}

	// Ensure storage path exists
	if err := agentConfig.initStorageDirectory(); err != nil {
		return nil, fmt.Errorf("failed to ensure storage path: %v", err)
	}

	log.Logger.Info("Agent configuration loaded successfully")
	return agentConfig, nil
}
