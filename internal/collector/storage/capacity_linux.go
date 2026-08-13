//go:build linux

package storage

import (
	"syscall"

	"github.com/arpingblue/AIStat/internal/model"
)

func fillCapacity(mount *model.Mount) {
	var stat syscall.Statfs_t
	if syscall.Statfs(mount.Target, &stat) != nil {
		return
	}
	mount.TotalBytes = stat.Blocks * uint64(stat.Bsize)
	mount.FreeBytes = stat.Bavail * uint64(stat.Bsize)
}
