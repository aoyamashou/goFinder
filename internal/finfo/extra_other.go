//go:build !darwin

package finfo

import "errors"

var errNoDarwin = errors.New("仅 macOS 支持")

func readExtra(in *Info) {}

func SetHideExt(path string, on bool) error { return errNoDarwin }
func SetHidden(path string, on bool) error  { return errNoDarwin }
func SetLocked(path string, on bool) error  { return errNoDarwin }
