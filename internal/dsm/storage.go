package dsm

import (
	"context"
	"net/url"
)

// Storage is the aggregate response from SYNO.Core.Storage.
type Storage struct {
	Volumes      []Volume      `json:"volumes"`
	StoragePools []StoragePool `json:"storagePools"`
	Disks        []Disk        `json:"disks"`
	HotSpares    []any         `json:"hotSpares,omitempty"`
	SSDCaches    []any         `json:"ssdCaches,omitempty"`
	Env          struct {
		Batchtask struct {
			RemainingTasks int `json:"remaining_tasks"`
		} `json:"batchtask"`
		ShareCryptoSupport bool `json:"share_crypto_support"`
		BTRFSReplicaSupport bool `json:"support_btrfs_replica"`
		ISCSILunSupport    bool `json:"support_iscsi_lun"`
	} `json:"env,omitempty"`
}

// Volume is a logical filesystem mounted on a pool.
type Volume struct {
	ID            string `json:"id"`            // e.g. "/volume1"
	Container     string `json:"container"`     // "pool"
	DeviceType    string `json:"device_type"`   // "shr_without_disk_protect", "raid5", …
	DisplayName   string `json:"display_name"`
	FSType        string `json:"fs_type"`       // "btrfs", "ext4"
	NumID         int    `json:"num_id"`
	PoolPath      string `json:"pool_path,omitempty"`
	RaidType      string `json:"raid_type,omitempty"`
	Size          struct {
		FreeInode  string `json:"free_inode"`
		TotalInode string `json:"total_inode"`
		Total      string `json:"total"`
		Used       string `json:"used"`
	} `json:"size"`
	Status       string `json:"status"`     // "normal", "degrade", "crashed"
	Progress     int    `json:"progress,omitempty"`
	SpacePath    string `json:"space_path,omitempty"`
}

// StoragePool groups physical disks into a redundancy group.
type StoragePool struct {
	ID         string   `json:"id"`
	DeviceType string   `json:"device_type"`
	RaidType   string   `json:"raid_type,omitempty"`
	NumID      int      `json:"num_id"`
	Disks      []string `json:"disks,omitempty"`
	Pool struct {
		Status string `json:"status"`
	} `json:"pool"`
	Size struct {
		Total string `json:"total"`
		Used  string `json:"used"`
	} `json:"size"`
	Progress int    `json:"progress,omitempty"`
	Status   string `json:"status"`
}

// Disk is a physical drive.
type Disk struct {
	ID          string `json:"id"`           // disk_unc id, e.g. /dev/sda
	Path        string `json:"path,omitempty"`
	Device      string `json:"device,omitempty"`
	DiskType    string `json:"diskType"`     // "SATA", "SAS", "M2_NVMe" …
	Model       string `json:"model"`
	Vendor      string `json:"vendor"`
	Firmware    string `json:"firm,omitempty"`
	Status      string `json:"status"`       // "normal", "warning", "critical"
	Temperature int    `json:"temp"`         // celsius
	Capacity    string `json:"capacity"`     // bytes as string
	Used        string `json:"used,omitempty"`
	Container   struct {
		Order int    `json:"order"`
		Type  string `json:"type"` // "internal", "ebox", "usb"
		Str   string `json:"str,omitempty"`
	} `json:"container"`
	Smart struct {
		Status string `json:"status,omitempty"`
	} `json:"smart,omitempty"`
	Used4K       bool   `json:"is_4kn_disk,omitempty"`
	Unused       bool   `json:"unused,omitempty"`
	BadSectors   int    `json:"bad_sector,omitempty"`
	NumID        int    `json:"num_id,omitempty"`
	Serial       string `json:"serial,omitempty"`
}

// Storage returns volumes/pools/disks in a single call.
func (c *Client) Storage(ctx context.Context) (*Storage, error) {
	var out Storage
	params := url.Values{}
	params.Set("action", "load_info")
	if err := c.Call(ctx, "SYNO.Core.Storage", 1, "load_info", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
