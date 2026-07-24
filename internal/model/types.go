package model

import "encoding/json"

const (
	APIVersion   = "storage-plugins.pasturestack.io/v1alpha1"
	RequestKind  = "LifecyclePlanRequest"
	MaxInputSize = 2 * 1024 * 1024
	MaxJSONDepth = 16
)

type DriverID string

const (
	DriverAliyunBlock DriverID = "aliyun-block"
	DriverAWSEBS      DriverID = "aws-ebs"
	DriverAWSEFS      DriverID = "aws-efs"
	DriverNFS         DriverID = "nfs"
	DriverCephRBD     DriverID = "ceph-rbd"
	DriverLonghorn    DriverID = "longhorn"
	DriverLoop        DriverID = "loop"
)

type Operation string

const (
	OperationCreate  Operation = "create"
	OperationAttach  Operation = "attach"
	OperationMount   Operation = "mount"
	OperationUnmount Operation = "unmount"
	OperationDetach  Operation = "detach"
	OperationRemove  Operation = "remove"
)

type Phase string

const (
	PhaseAbsent    Phase = "absent"
	PhaseAvailable Phase = "available"
	PhaseAttached  Phase = "attached"
	PhaseMounted   Phase = "mounted"
)

type Request struct {
	APIVersion         string        `json:"apiVersion"`
	Kind               string        `json:"kind"`
	Driver             DriverID      `json:"driver"`
	Operation          Operation     `json:"operation"`
	IdempotencyKey     string        `json:"idempotencyKey"`
	OwnerID            string        `json:"ownerId"`
	ExpectedGeneration uint64        `json:"expectedGeneration"`
	ObservedState      ObservedState `json:"observedState"`
	Desired            Desired       `json:"desired"`
}

type ObservedState struct {
	Phase       Phase  `json:"phase"`
	Generation  uint64 `json:"generation"`
	OwnerID     string `json:"ownerId"`
	ResourceRef string `json:"resourceRef,omitempty"`
}

type Desired struct {
	Phase             Phase        `json:"phase"`
	AllowIrreversible bool         `json:"allowIrreversible"`
	Config            DriverConfig `json:"config"`
}

type DriverConfig interface {
	DriverID() DriverID
}

type AliyunBlockConfig struct {
	DiskCategory string `json:"diskCategory"`
	SizeGiB      uint64 `json:"sizeGiB"`
	SnapshotRef  string `json:"snapshotRef,omitempty"`
}

func (AliyunBlockConfig) DriverID() DriverID { return DriverAliyunBlock }

type AWSEBSConfig struct {
	VolumeType  string `json:"volumeType"`
	SizeGiB     uint64 `json:"sizeGiB"`
	IOPS        uint64 `json:"iops,omitempty"`
	SnapshotRef string `json:"snapshotRef,omitempty"`
	Encrypted   bool   `json:"encrypted"`
	KMSRef      string `json:"kmsRef,omitempty"`
}

func (AWSEBSConfig) DriverID() DriverID { return DriverAWSEBS }

type AWSEFSConfig struct {
	PerformanceMode string   `json:"performanceMode"`
	ExportRef       string   `json:"exportRef"`
	MountOptions    []string `json:"mountOptions,omitempty"`
}

func (AWSEFSConfig) DriverID() DriverID { return DriverAWSEFS }

type NFSConfig struct {
	ServerRef    string   `json:"serverRef"`
	ExportRef    string   `json:"exportRef"`
	MountOptions []string `json:"mountOptions,omitempty"`
	PurgePolicy  string   `json:"purgePolicy"`
}

func (NFSConfig) DriverID() DriverID { return DriverNFS }

type CephRBDConfig struct {
	Pool         string `json:"pool"`
	SizeGiB      uint64 `json:"sizeGiB"`
	ImageFeature string `json:"imageFeature"`
}

func (CephRBDConfig) DriverID() DriverID { return DriverCephRBD }

type LonghornConfig struct {
	SizeBytes uint64 `json:"sizeBytes"`
	Replicas  uint64 `json:"replicas"`
}

func (LonghornConfig) DriverID() DriverID { return DriverLonghorn }

type LoopConfig struct {
	SizeMiB uint64 `json:"sizeMiB"`
}

func (LoopConfig) DriverID() DriverID { return DriverLoop }

type Controls struct {
	Network        bool `json:"network"`
	CloudAPI       bool `json:"cloudApi"`
	DockerSocket   bool `json:"dockerSocket"`
	HostSocket     bool `json:"hostSocket"`
	HostPath       bool `json:"hostPath"`
	Privileged     bool `json:"privileged"`
	Mount          bool `json:"mount"`
	Unmount        bool `json:"unmount"`
	Format         bool `json:"format"`
	Delete         bool `json:"delete"`
	CredentialRead bool `json:"credentialRead"`
	SecretRead     bool `json:"secretRead"`
	StateWrite     bool `json:"stateWrite"`
	Execution      bool `json:"execution"`
}

func DisabledControls() Controls { return Controls{} }

type Operations struct {
	Create  bool `json:"create"`
	Attach  bool `json:"attach"`
	Mount   bool `json:"mount"`
	Unmount bool `json:"unmount"`
	Detach  bool `json:"detach"`
	Remove  bool `json:"remove"`
}

type ContractTraits struct {
	AttachMode            string `json:"attachMode"`
	SnapshotSource        bool   `json:"snapshotSource"`
	FilesystemPreparation bool   `json:"filesystemPreparation"`
	PurgePolicy           bool   `json:"purgePolicy"`
}

type DriverCapability struct {
	Driver          DriverID       `json:"driver"`
	DevelopmentOnly bool           `json:"developmentOnly"`
	Operations      Operations     `json:"operations"`
	Traits          ContractTraits `json:"traits"`
	WouldRequire    []string       `json:"wouldRequire"`
}

type DelegatedComponent struct {
	Repository string `json:"repository"`
	Included   bool   `json:"included"`
	ReasonCode string `json:"reasonCode"`
}

type CapabilitiesOutput struct {
	APIVersion          string               `json:"apiVersion"`
	Kind                string               `json:"kind"`
	Locale              string               `json:"locale"`
	Drivers             []DriverCapability   `json:"drivers"`
	DelegatedComponents []DelegatedComponent `json:"delegatedComponents"`
	Controls            Controls             `json:"controls"`
}

type Diagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type ValidationOutput struct {
	APIVersion  string       `json:"apiVersion"`
	Kind        string       `json:"kind"`
	Locale      string       `json:"locale"`
	Valid       bool         `json:"valid"`
	Driver      DriverID     `json:"driver"`
	Operation   Operation    `json:"operation"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	Controls    Controls     `json:"controls"`
}

type Gate struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Code   string `json:"code"`
}

type PlanStep struct {
	Sequence     int      `json:"sequence"`
	Action       string   `json:"action"`
	Status       string   `json:"status"`
	Irreversible bool     `json:"irreversible"`
	WouldRequire []string `json:"wouldRequire"`
	ReasonCode   string   `json:"reasonCode"`
}

type PlanOutput struct {
	APIVersion     string       `json:"apiVersion"`
	Kind           string       `json:"kind"`
	Locale         string       `json:"locale"`
	PlanID         string       `json:"planId"`
	Driver         DriverID     `json:"driver"`
	Operation      Operation    `json:"operation"`
	ObservedPhase  Phase        `json:"observedPhase"`
	RequestedPhase Phase        `json:"requestedPhase"`
	ValidIntent    bool         `json:"validIntent"`
	Executable     bool         `json:"executable"`
	Effect         string       `json:"effect"`
	Steps          []PlanStep   `json:"steps"`
	Gates          []Gate       `json:"gates"`
	Diagnostics    []Diagnostic `json:"diagnostics"`
	Controls       Controls     `json:"controls"`
}

type ErrorOutput struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

type CanonicalRequest struct {
	APIVersion         string        `json:"apiVersion"`
	Kind               string        `json:"kind"`
	Driver             DriverID      `json:"driver"`
	Operation          Operation     `json:"operation"`
	IdempotencyKey     string        `json:"idempotencyKey"`
	OwnerID            string        `json:"ownerId"`
	ExpectedGeneration uint64        `json:"expectedGeneration"`
	ObservedState      ObservedState `json:"observedState"`
	Desired            Desired       `json:"desired"`
}

func (r Request) CanonicalJSON() ([]byte, error) {
	return json.Marshal(CanonicalRequest(r))
}
