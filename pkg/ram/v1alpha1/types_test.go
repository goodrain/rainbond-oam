package v1alpha1

import (
	"encoding/json"
	"testing"
)

func TestComponentHandleNullValueInitializesVMMetadata(t *testing.T) {
	component := &Component{
		ServiceType: "vm",
		VM: &VMTemplate{
			BootMode:         "bios",
			MachineType:      "q35",
			BootSourceFormat: "qcow2",
		},
	}

	component.HandleNullValue()

	if component.VM == nil {
		t.Fatalf("expected vm metadata to be preserved")
	}
	if component.VM.DiskLayout == nil {
		t.Fatalf("expected vm disk layout to be initialized")
	}
}

func TestComponentValidationRequiresVMRootDisk(t *testing.T) {
	component := &Component{
		ServiceType: "vm",
		VM: &VMTemplate{
			BootMode:         "bios",
			MachineType:      "q35",
			BootSourceFormat: "qcow2",
			DiskLayout: []VMDiskLayoutItem{
				{
					DiskKey:     "data-1",
					DiskName:    "data-1",
					DiskRole:    VMDiskRoleData,
					DeviceType:  VMDiskDeviceDisk,
					Bus:         "sata",
					OrderIndex:  0,
					VolumeName:  "data-1",
					RequestSize: "20Gi",
					Format:      "qcow2",
					SourceType:  VMDiskSourceRegistry,
					Image:       "registry.example.com/test/data-1:v1",
				},
			},
		},
	}

	if err := component.Validation(); err == nil {
		t.Fatalf("expected vm validation to fail without root disk")
	}
}

func TestRainbondApplicationConfigJSONIncludesVMMetadata(t *testing.T) {
	config := &RainbondApplicationConfig{
		AppKeyID:        "app-key",
		AppName:         "vm-app",
		AppVersion:      "1.0.0",
		TempleteVersion: "v3",
		Components: []*Component{
			{
				ServiceType:  "vm",
				ServiceName:  "windows-vm",
				ServiceAlias: "windows-vm",
				VM: &VMTemplate{
					BootMode:         "bios",
					MachineType:      "q35",
					BootSourceFormat: "qcow2",
					DiskLayout: []VMDiskLayoutItem{
						{
							DiskKey:     "disk",
							DiskName:    "system-disk",
							DiskRole:    VMDiskRoleRoot,
							DeviceType:  VMDiskDeviceDisk,
							Bus:         "sata",
							OrderIndex:  0,
							VolumeName:  "disk",
							RequestSize: "80Gi",
							Format:      "qcow2",
							SourceType:  VMDiskSourceRegistry,
							Image:       "registry.example.com/test/windows-vm:v1",
							Checksum:    "sha256:test",
						},
					},
				},
			},
		},
	}

	config.HandleNullValue()

	body := config.JSON()

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("unmarshal config json: %v", err)
	}

	apps, ok := payload["apps"].([]interface{})
	if !ok || len(apps) != 1 {
		t.Fatalf("expected exactly one component in payload")
	}

	component, ok := apps[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected component payload to be an object")
	}

	vmPayload, ok := component["vm"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected vm metadata to be serialized")
	}

	if vmPayload["boot_mode"] != "bios" {
		t.Fatalf("expected boot_mode=bios, got %v", vmPayload["boot_mode"])
	}

	diskLayout, ok := vmPayload["disk_layout"].([]interface{})
	if !ok || len(diskLayout) != 1 {
		t.Fatalf("expected one vm disk layout item")
	}
}

func TestRainbondApplicationConfigHandleNullValuePromotesVMTemplateVersion(t *testing.T) {
	config := &RainbondApplicationConfig{
		Components: []*Component{
			{
				ServiceType: "vm",
				VM: &VMTemplate{
					BootMode:         "bios",
					MachineType:      "q35",
					BootSourceFormat: "qcow2",
					DiskLayout: []VMDiskLayoutItem{
						{
							DiskKey:     "disk",
							DiskRole:    VMDiskRoleRoot,
							DeviceType:  VMDiskDeviceDisk,
							SourceType:  VMDiskSourceRegistry,
							RequestSize: "80Gi",
						},
					},
				},
			},
		},
	}

	config.HandleNullValue()

	if config.TempleteVersion != "v3" {
		t.Fatalf("expected vm template version to promote to v3, got %s", config.TempleteVersion)
	}
}
