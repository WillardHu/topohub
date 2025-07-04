package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="SUBNET",type="string",JSONPath=".spec.ipv4Subnet.subnet"
// +kubebuilder:printcolumn:name="SERVER_IP",type="string",JSONPath=".spec.interface.ipv4"
// +kubebuilder:printcolumn:name="IP_TOTAL",type="integer",JSONPath=".status.dhcpStatus.dhcpIpTotalAmount"
// +kubebuilder:printcolumn:name="IP_AVAILABLE",type="integer",JSONPath=".status.dhcpStatus.dhcpIpAvailableAmount"
// +kubebuilder:printcolumn:name="IP_RESERVED",type="integer",JSONPath=".status.dhcpStatus.dhcpIpBindAmount"
// +kubebuilder:printcolumn:name="SYNC_redfishStatus",type="boolean",JSONPath=".spec.feature.syncRedfishstatus.enabled"
// +kubebuilder:printcolumn:name="PXE",type="boolean",JSONPath=".spec.feature.enablePxe"
// +kubebuilder:printcolumn:name="ZTP",type="boolean",JSONPath=".spec.feature.enableZtp"
// +kubebuilder:subresource:status

// Subnet is the Schema for the subnets API
type Subnet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SubnetSpec   `json:"spec,omitempty"`
	Status SubnetStatus `json:"status,omitempty"`
}

// IPv4SubnetSpec defines the IPv4 subnet configuration
type IPv4SubnetSpec struct {
	// Subnet for DHCP server (required)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}/([0-9]|[1-2][0-9]|3[0-2])$`
	Subnet string `json:"subnet"`

	// IPRange for DHCP server (required)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}-([0-9]{1,3}\.){3}[0-9]{1,3}(,([0-9]{1,3}\.){3}[0-9]{1,3}-([0-9]{1,3}\.){3}[0-9]{1,3})*$`
	IPRange string `json:"ipRange"`

	// Gateway for DHCP server (optional)
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}$`
	// +optional
	Gateway *string `json:"gateway,omitempty"`

	// DNS server (optional)
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}$`
	// +optional
	Dns *string `json:"dns,omitempty"`
}

// InterfaceSpec defines the network interface configuration
type InterfaceSpec struct {
	// DHCP server interface (required)
	// +kubebuilder:validation:Required
	Interface string `json:"interface"`

	// VLAN ID (optional, 0-4094)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4094
	// +optional
	VlanID *int32 `json:"vlanId,omitempty"`

	// Self IP for DHCP server (required)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}/([0-9]|[1-2][0-9]|3[0-2])$`
	IPv4 string `json:"ipv4"`
}

// FeatureSpec defines the feature configuration
type FeatureSpec struct {
	// SyncRedfishstatus configuration
	SyncRedfishstatus SyncRedfishstatusSpec `json:"syncRedfishstatus"`

	// Enable PXE boot support
	// +kubebuilder:validation:Required
	// +kubebuilder:default=false
	EnablePxe bool `json:"enablePxe"`

	// Enable ZTP configuration for switch
	// +kubebuilder:validation:Required
	// +kubebuilder:default=false
	EnableZtp bool `json:"enableZtp"`

	// Enable DHCP trusted only mode (dhcp-ignore=tag:!trusted)
	// +kubebuilder:default=false
	// +optional
	EnableDhcpTrustedOnly bool `json:"enableDhcpTrustedOnly"`
}

// SyncRedfishstatusSpec defines the sync endpoint configuration
type SyncRedfishstatusSpec struct {
	// Enable automatically create the redfishstatus object for the dhcp client. Notice, it will not be deleted automatically
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// Enable bind dhcp ip
	// +kubebuilder:validation:Required
	// +kubebuilder:default=true
	EnableBindDhcpIP bool `json:"enableBindDhcpIP"`

	// Default cluster name
	// +optional
	DefaultClusterName *string `json:"defaultClusterName,omitempty"`
}

// SubnetSpec defines the desired state of Subnet
type SubnetSpec struct {
	// IPv4Subnet configuration
	// +kubebuilder:validation:Required
	IPv4Subnet IPv4SubnetSpec `json:"ipv4Subnet"`

	// Interface configuration
	// +kubebuilder:validation:Required
	Interface InterfaceSpec `json:"interface"`

	// Feature configuration
	// +optional
	Feature *FeatureSpec `json:"feature,omitempty"`
}

// SubnetStatus defines the observed state of Subnet
type SubnetStatus struct {
	// Dhcp ip status
	// +optional
	DhcpStatus *DhcpStatusSpec `json:"dhcpStatus,omitempty"`

	// the name of the node who hosts the subnet
	HostNode *string `json:"hostNode,omitempty"`

	// Conditions represent the latest available observations of an object's state
	// +optional
	// +listType=map
	// +listMapKey=type
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Dhcp client details
	DhcpClientDetails string `json:"dhcpClientDetails"`
}

type DhcpStatusSpec struct {
	// Total number of IP addresses in the subnet
	DhcpIpTotalAmount uint64 `json:"dhcpIpTotalAmount"`

	// Number of available IP addresses
	DhcpIpAvailableAmount uint64 `json:"dhcpIpAvailableAmount"`

	// Number of assigned IP addresses which is in use in the lease file
	DhcpIpActiveAmount uint64 `json:"dhcpIpActiveAmount"`

	// Number of reserved IP addresses which is bond to MAC address
	DhcpIpBindAmount uint64 `json:"dhcpIpBindAmount"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SubnetList contains a list of Subnet
type SubnetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Subnet `json:"items"`
}
