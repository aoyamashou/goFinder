package gui

import (
	"fmt"
	"os"

	"gofinder/internal/fileundo"
)

type undoStep struct {
	entries     []fileundo.Entry
	unavailable int
}

// These counters and the history are owned exclusively by the UI thread.
func (a *app) beginFileOp() bool {
	if a.undoing {
		a.feedback("正在撤销文件操作")
		return false
	}
	a.fileOps++
	return true
}

func (a *app) finishFileOp(step undoStep) {
	a.fileOps--
	if len(step.entries) == 0 && step.unavailable == 0 {
		return
	}
	a.undoSteps = append(a.undoSteps, step)
	if len(a.undoSteps) > 50 {
		a.undoSteps = a.undoSteps[len(a.undoSteps)-50:]
	}
	if step.unavailable > 0 {
		a.status += fmt.Sprintf("；%d 项无法撤销", step.unavailable)
	}
}

func sourceInfo(paths []string) []os.FileInfo {
	infos := make([]os.FileInfo, len(paths))
	for i, p := range paths {
		infos[i], _ = os.Lstat(p)
	}
	return infos
}

// transfers returns only inverses for completed outputs, including a partial
// batch. Cross-volume moves cannot retain identity and form an explicit barrier.
func transfers(srcs, out []string, before []os.FileInfo, move bool) undoStep {
	var step undoStep
	for i, dst := range out {
		original := ""
		if move {
			if srcs[i] == dst {
				continue
			}
			original = srcs[i]
			if !fileundo.Same(dst, before[i]) {
				step.unavailable++
				continue
			}
		}
		e, err := fileundo.Record(dst, original)
		if err != nil {
			step.unavailable++
			continue
		}
		step.entries = append(step.entries, e)
	}
	return step
}

func (a *app) undoFiles() {
	if a.fileOps > 0 || a.undoing {
		a.feedback("有文件操作进行中")
		return
	}
	if len(a.undoSteps) == 0 {
		a.feedback("没有可撤销的文件操作")
		return
	}
	step := a.undoSteps[len(a.undoSteps)-1]
	if len(step.entries) == 0 {
		a.feedback(fmt.Sprintf("其余 %d 项无法撤销，更早的操作已不可撤销", step.unavailable))
		return
	}
	a.undoing = true
	sys := a.sys
	a.async(func() {
		n := 0
		var failure error
		for len(step.entries) > 0 {
			i := len(step.entries) - 1
			if err := step.entries[i].Undo(sys.Trash); err != nil {
				failure = err
				break
			}
			step.entries = step.entries[:i]
			n++
		}
		a.post(func() {
			a.undoing = false
			i := len(a.undoSteps) - 1
			a.undoSteps[i] = step
			if len(step.entries) == 0 && step.unavailable == 0 {
				a.undoSteps = a.undoSteps[:i]
			}
			a.status = fmt.Sprintf("已撤销 %d 项", n)
			if failure != nil {
				a.status += "；失败：" + failure.Error()
			}
			if step.unavailable > 0 {
				a.status += fmt.Sprintf("；其余 %d 项无法撤销，更早的操作已不可撤销", step.unavailable)
			}
			a.feedback(a.status)
			a.refreshLists()
		})
	})
}

// Called after folder initialization has finished; metadata I/O stays off-thread.
func (a *app) recordCreated(path string, ok bool) {
	a.async(func() {
		step := undoStep{unavailable: 1}
		if ok {
			if e, err := fileundo.Record(path, ""); err == nil {
				step = undoStep{entries: []fileundo.Entry{e}}
			}
		}
		a.post(func() { a.finishFileOp(step) })
	})
}
