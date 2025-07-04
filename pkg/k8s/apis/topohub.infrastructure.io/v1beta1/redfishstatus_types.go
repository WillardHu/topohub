package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="CLUSTERNAME",type="string",JSONPath=".status.basic.clusterName"
// +kubebuilder:printcolumn:name="HEALTHY",type="boolean",JSONPath=".status.healthy"
// +kubebuilder:printcolumn:name="IPADDR",type="string",JSONPath=".status.basic.ipAddr"
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".status.basic.type"
// +kubebuilder:printcolumn:name="WARNING",type="string",JSONPath=".status.log.warningLogAccount"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

type RedfishStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status RedfishStatusStatus `json:"status,omitempty"`
}

type RedfishStatusStatus struct {
	Healthy        bool              `json:"healthy"`
	LastUpdateTime string            `json:"lastUpdateTime"`
	Basic          BasicInfo         `json:"basic"`
	Info           map[string]string `json:"info"`
	Log            LogStruct         `json:"log"`
}

type LogStruct struct {
	// +kubebuilder:validation:Required
	TotalLogAccount   int32 `json:"totalLogAccount"`
	WarningLogAccount int32 `json:"warningLogAccount"`
	// +optional
	LastestLog *LogEntry `json:"lastestLog,omitempty"`
	// +optional
	LastestWarningLog *LogEntry `json:"lastestWarningLog,omitempty"`
}

type LogEntry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
}

type BasicInfo struct {
	ClusterName     string `json:"clusterName"`
	Type            string `json:"type"`
	IpAddr          string `json:"ipAddr"`
	SecretName      string `json:"secretName"`
	SecretNamespace string `json:"secretNamespace"`
	Https           bool   `json:"https"`
	Port            int32  `json:"port"`
	Mac             string `json:"mac,omitempty"`
	// ActiveDhcpClient specifies this host is an active dhcp client when type is dhcp
	// +optional
	ActiveDhcpClient bool    `json:"activeDhcpClient,omitempty"`
	DhcpExpireTime   *string `json:"dhcpExpireTime,omitempty"`
	SubnetName       *string `json:"subnetName,omitempty"`
	Hostname         *string `json:"hostname,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RedfishStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RedfishStatus `json:"items"`
}
