package applecontainer

import (
	"bytes"
	"encoding/json"
	"strconv"
	"time"
)

type AppleDate time.Time

func (d *AppleDate) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}
	seconds, err := strconv.ParseFloat(string(data), 64)
	if err != nil {
		return err
	}
	whole := int64(seconds)
	*d = AppleDate(time.Unix(978307200+whole, int64((seconds-float64(whole))*1e9)).UTC())
	return nil
}

func (d AppleDate) MarshalJSON() ([]byte, error) {
	t := time.Time(d).UTC()
	if t.IsZero() {
		return []byte("null"), nil
	}
	seconds := float64(t.Unix()-978307200) + float64(t.Nanosecond())/1e9
	return []byte(strconv.FormatFloat(seconds, 'f', -1, 64)), nil
}

type ContainerConfiguration struct {
	ID               string               `json:"id"`
	Image            ImageDescription     `json:"image"`
	Mounts           []Filesystem         `json:"mounts,omitempty"`
	PublishedPorts   []PublishPort        `json:"publishedPorts,omitempty"`
	PublishedSockets []map[string]any     `json:"publishedSockets,omitempty"`
	Labels           map[string]string    `json:"labels,omitempty"`
	Sysctls          map[string]string    `json:"sysctls,omitempty"`
	Networks         []AttachmentConfig   `json:"networks,omitempty"`
	DNS              *DNSConfiguration    `json:"dns,omitempty"`
	Rosetta          bool                 `json:"rosetta,omitempty"`
	InitProcess      ProcessConfiguration `json:"initProcess"`
	Platform         *OCIPlatform         `json:"platform,omitempty"`
	Resources        Resources            `json:"resources,omitempty"`
	RuntimeHandler   string               `json:"runtimeHandler,omitempty"`
	Virtualization   bool                 `json:"virtualization,omitempty"`
	SSH              bool                 `json:"ssh,omitempty"`
	ReadOnly         bool                 `json:"readOnly,omitempty"`
	UseInit          bool                 `json:"useInit,omitempty"`
	CapAdd           []string             `json:"capAdd,omitempty"`
	CapDrop          []string             `json:"capDrop,omitempty"`
	ShmSize          *uint64              `json:"shmSize,omitempty"`
	StopSignal       *string              `json:"stopSignal,omitempty"`
	CreationDate     *AppleDate           `json:"creationDate,omitempty"`
}

type ImageDescription struct {
	Reference  string     `json:"reference"`
	Descriptor Descriptor `json:"descriptor"`
}

type Descriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	URLs        []string          `json:"urls,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Platform    *OCIPlatform      `json:"platform,omitempty"`
}

type OCIPlatform struct {
	Architecture string   `json:"architecture"`
	OS           string   `json:"os"`
	OSVersion    string   `json:"os.version,omitempty"`
	OSFeatures   []string `json:"os.features,omitempty"`
	Variant      string   `json:"variant,omitempty"`
}

type SystemPlatform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
}

type ProcessConfiguration struct {
	Executable         string   `json:"executable"`
	Arguments          []string `json:"arguments"`
	Environment        []string `json:"environment"`
	WorkingDirectory   string   `json:"workingDirectory"`
	Terminal           bool     `json:"terminal"`
	User               any      `json:"user"`
	SupplementalGroups []uint32 `json:"supplementalGroups"`
	Rlimits            []any    `json:"rlimits"`
}

type PublishPort struct {
	HostAddress   string `json:"hostAddress"`
	HostPort      uint16 `json:"hostPort"`
	ContainerPort uint16 `json:"containerPort"`
	Proto         string `json:"proto"`
	Count         uint16 `json:"count"`
}

type DNSConfiguration struct {
	Nameservers   []string `json:"nameservers"`
	Domain        string   `json:"domain,omitempty"`
	SearchDomains []string `json:"searchDomains"`
	Options       []string `json:"options"`
}

type Resources struct {
	CPUs        int    `json:"cpus"`
	MemoryBytes uint64 `json:"memoryInBytes"`
	Storage     uint64 `json:"storage,omitempty"`
	CPUOverhead int    `json:"cpuOverhead,omitempty"`
}

type AttachmentConfig struct {
	Network string            `json:"network"`
	Options AttachmentOptions `json:"options"`
}

type AttachmentOptions struct {
	Hostname   string `json:"hostname"`
	MACAddress any    `json:"macAddress,omitempty"`
	MTU        uint32 `json:"mtu,omitempty"`
}

type Filesystem struct {
	Type        map[string]any `json:"type"`
	Source      string         `json:"source"`
	Destination string         `json:"destination"`
	Options     []string       `json:"options"`
}

type ContainerSnapshot struct {
	Configuration ContainerConfiguration `json:"configuration"`
	Status        string                 `json:"status"`
	Networks      []map[string]any       `json:"networks"`
	StartedDate   *AppleDate             `json:"startedDate,omitempty"`
}

type ContainerListFilters struct {
	IDs    []string          `json:"ids"`
	Status string            `json:"status,omitempty"`
	Labels map[string]string `json:"labels"`
}

func (f ContainerListFilters) MarshalJSON() ([]byte, error) {
	ids := f.IDs
	if ids == nil {
		ids = []string{}
	}
	labels := f.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	type containerListFiltersJSON struct {
		IDs    []string          `json:"ids"`
		Status string            `json:"status,omitempty"`
		Labels map[string]string `json:"labels"`
	}
	return json.Marshal(containerListFiltersJSON{IDs: ids, Status: f.Status, Labels: labels})
}

type VolumeConfig struct {
	Name         string            `json:"name"`
	Driver       string            `json:"driver"`
	Format       string            `json:"format"`
	Source       string            `json:"source"`
	CreationDate AppleDate         `json:"creationDate"`
	Labels       map[string]string `json:"labels,omitempty"`
	Options      map[string]string `json:"options,omitempty"`
	SizeInBytes  *uint64           `json:"sizeInBytes,omitempty"`
}

type NetworkResource struct {
	Configuration NetworkConfiguration `json:"configuration"`
	Status        map[string]any       `json:"status,omitempty"`
}

type NetworkConfiguration struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
}

func ProcessUserRoot() any {
	return map[string]any{
		"id": map[string]uint32{"uid": 0, "gid": 0},
	}
}

func enumNoValue(name string) map[string]any {
	return map[string]any{name: map[string]any{}}
}

func VolumeFilesystem(name, format, source, destination string, readOnly bool) Filesystem {
	options := []string{}
	if readOnly {
		options = append(options, "ro")
	}
	return Filesystem{
		Type: map[string]any{
			"volume": map[string]any{
				"name":   name,
				"format": format,
				"cache":  enumNoValue("on"),
				"sync":   enumNoValue("fsync"),
			},
		},
		Source:      source,
		Destination: destination,
		Options:     options,
	}
}
