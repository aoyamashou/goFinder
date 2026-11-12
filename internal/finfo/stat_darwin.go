package finfo

import (
	"syscall"
	"time"
)

const (
	ufImmutable = 0x2    // UF_IMMUTABLE
	ufHidden    = 0x8000 // UF_HIDDEN
)

func fill(in *Info, st *syscall.Stat_t) {
	in.Created = time.Unix(st.Birthtimespec.Unix())
	in.Accessed = time.Unix(st.Atimespec.Unix())
	in.Owner = userName(st.Uid)
	in.Group = groupName(uint32(st.Gid))
	in.Hidden = st.Flags&ufHidden != 0
	in.Locked = st.Flags&ufImmutable != 0
}
