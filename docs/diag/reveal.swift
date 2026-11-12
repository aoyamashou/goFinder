// 诊断「在 Finder 中显示」：只发 reveal 请求，不改任何系统设置。
// 用法：swift docs/diag/reveal.swift /path/to/file
import AppKit
let path = CommandLine.arguments.count > 1 ? CommandLine.arguments[1] : NSHomeDirectory()
let url = URL(fileURLWithPath: path)
print("activateFileViewerSelecting:", path)
NSWorkspace.shared.activateFileViewerSelecting([url])
print("selectFile:inFileViewerRootedAtPath:")
NSWorkspace.shared.selectFile(path, inFileViewerRootedAtPath: "")
RunLoop.current.run(until: Date().addingTimeInterval(1))
