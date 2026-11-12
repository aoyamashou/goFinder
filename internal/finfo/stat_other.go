//go:build !darwin

package finfo

import "syscall"

func fill(in *Info, st *syscall.Stat_t) {
	in.Owner = userName(st.Uid)
	in.Group = groupName(uint32(st.Gid))
}
