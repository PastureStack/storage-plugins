package catalog

import "github.com/PastureStack/storage-plugins/internal/model"

var drivers = []model.DriverCapability{
	{
		Driver:          model.DriverAliyunBlock,
		DevelopmentOnly: false,
		Operations:      allOperations(),
		Traits: model.ContractTraits{
			AttachMode:            "device",
			SnapshotSource:        true,
			FilesystemPreparation: true,
			PurgePolicy:           false,
		},
		WouldRequire: []string{"cloud-api", "credential-read", "delete", "format", "host-path", "mount", "network", "privileged", "state-write", "unmount"},
	},
	{
		Driver:          model.DriverAWSEBS,
		DevelopmentOnly: false,
		Operations:      allOperations(),
		Traits: model.ContractTraits{
			AttachMode:            "device",
			SnapshotSource:        true,
			FilesystemPreparation: true,
			PurgePolicy:           false,
		},
		WouldRequire: []string{"cloud-api", "credential-read", "delete", "format", "host-path", "mount", "network", "privileged", "state-write", "unmount"},
	},
	{
		Driver:          model.DriverAWSEFS,
		DevelopmentOnly: false,
		Operations:      allOperations(),
		Traits: model.ContractTraits{
			AttachMode:            "logical",
			SnapshotSource:        false,
			FilesystemPreparation: false,
			PurgePolicy:           false,
		},
		WouldRequire: []string{"cloud-api", "credential-read", "delete", "mount", "network", "privileged", "state-write", "unmount"},
	},
	{
		Driver:          model.DriverCephRBD,
		DevelopmentOnly: false,
		Operations:      allOperations(),
		Traits: model.ContractTraits{
			AttachMode:            "device",
			SnapshotSource:        false,
			FilesystemPreparation: true,
			PurgePolicy:           false,
		},
		WouldRequire: []string{"delete", "format", "host-path", "mount", "network", "privileged", "state-write", "unmount"},
	},
	{
		Driver:          model.DriverLonghorn,
		DevelopmentOnly: false,
		Operations:      allOperations(),
		Traits: model.ContractTraits{
			AttachMode:            "device",
			SnapshotSource:        false,
			FilesystemPreparation: true,
			PurgePolicy:           false,
		},
		WouldRequire: []string{"delete", "format", "host-path", "host-socket", "mount", "privileged", "state-write", "unmount"},
	},
	{
		Driver:          model.DriverLoop,
		DevelopmentOnly: true,
		Operations: model.Operations{
			Create:  true,
			Attach:  true,
			Mount:   false,
			Unmount: false,
			Detach:  true,
			Remove:  true,
		},
		Traits: model.ContractTraits{
			AttachMode:            "device",
			SnapshotSource:        false,
			FilesystemPreparation: false,
			PurgePolicy:           false,
		},
		WouldRequire: []string{"delete", "host-path", "privileged", "state-write"},
	},
	{
		Driver:          model.DriverNFS,
		DevelopmentOnly: false,
		Operations:      allOperations(),
		Traits: model.ContractTraits{
			AttachMode:            "logical",
			SnapshotSource:        false,
			FilesystemPreparation: false,
			PurgePolicy:           true,
		},
		WouldRequire: []string{"delete", "mount", "network", "privileged", "state-write", "unmount"},
	},
}

var delegated = []model.DelegatedComponent{
	{
		Repository: "PastureStack/secrets-flexvolume-plugin",
		Included:   false,
		ReasonCode: "delegated-component",
	},
	{
		Repository: "PastureStack/vault-secrets-bridge",
		Included:   false,
		ReasonCode: "delegated-component",
	},
}

func allOperations() model.Operations {
	return model.Operations{
		Create:  true,
		Attach:  true,
		Mount:   true,
		Unmount: true,
		Detach:  true,
		Remove:  true,
	}
}

func Drivers() []model.DriverCapability {
	result := make([]model.DriverCapability, len(drivers))
	copy(result, drivers)
	return result
}

func DelegatedComponents() []model.DelegatedComponent {
	result := make([]model.DelegatedComponent, len(delegated))
	copy(result, delegated)
	return result
}

func Output(locale string) model.CapabilitiesOutput {
	return model.CapabilitiesOutput{
		APIVersion:          model.APIVersion,
		Kind:                "StoragePluginCapabilities",
		Locale:              locale,
		Drivers:             Drivers(),
		DelegatedComponents: DelegatedComponents(),
		Controls:            model.DisabledControls(),
	}
}

func Lookup(driver model.DriverID) (model.DriverCapability, bool) {
	for _, candidate := range drivers {
		if candidate.Driver == driver {
			return candidate, true
		}
	}
	return model.DriverCapability{}, false
}

func SupportsOperation(capability model.DriverCapability, operation model.Operation) bool {
	switch operation {
	case model.OperationCreate:
		return capability.Operations.Create
	case model.OperationAttach:
		return capability.Operations.Attach
	case model.OperationMount:
		return capability.Operations.Mount
	case model.OperationUnmount:
		return capability.Operations.Unmount
	case model.OperationDetach:
		return capability.Operations.Detach
	case model.OperationRemove:
		return capability.Operations.Remove
	default:
		return false
	}
}
