package main

import (
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"

	"gofinder/internal/agents"
)

// agentItem is an agent to open a folder with: what menus and the a key
// list. Command is the command line to run, the user's own when they
// changed it, else the catalog's with its YOLO arguments.
type agentItem struct{ ID, Name, Symbol, Command string }

// agentCfg is an agent's entry in config.json. An agent without one is
// enabled with the default command.
type agentCfg struct {
	Enabled bool `json:"enabled"`
	// Command is the user's command line; "" is agents.DefaultCommand.
	Command string `json:"command,omitempty"`
}

// agentState is what the app knows of the installed agents.
type agentState struct {
	found    []agents.Found // installed, in catalog order
	loading  bool           // a detection is running
	seq      int            // drops the result of a superseded detection
	path     string         // the login shell's PATH, read once
	pathRead bool
	exists   func(string) bool // is it an executable file
	home     string
	def      string               // the default agent, as the settings' radio shows it
	rows     map[string]*agentRow // the settings page's edit state, by agent ID
}

// agentRow is one row of the settings' Agent page.
type agentRow struct {
	cmd string // the command box
	seg int    // 0 enabled, 1 disabled
}

func newAgentState() agentState {
	home, _ := os.UserHomeDir()
	return agentState{exists: agents.IsExecutable, home: home, rows: map[string]*agentRow{}}
}

// agentSymbol is the SF Symbol of an agent.
func agentSymbol(id string) string {
	if strings.HasPrefix(id, "claude") {
		return "sparkle"
	}
	return "terminal"
}

func (a *app) agentEnabled(id string) bool {
	if c, ok := a.cfg.Agents[id]; ok {
		return c.Enabled
	}
	return true
}

// agentCommand is the command line to run for e.
func (a *app) agentCommand(e agents.Entry) string {
	if c := strings.TrimSpace(a.cfg.Agents[e.ID].Command); c != "" {
		return c
	}
	return agents.DefaultCommand(e)
}

func (a *app) agentItemOf(e agents.Entry) agentItem {
	return agentItem{ID: e.ID, Name: e.Name, Symbol: agentSymbol(e.ID), Command: a.agentCommand(e)}
}

// enabledAgents lists the installed agents that are enabled, in the
// settings' order.
func (a *app) enabledAgents() []agentItem {
	var out []agentItem
	for _, f := range a.ag.found {
		if a.agentEnabled(f.Entry.ID) {
			out = append(out, a.agentItemOf(f.Entry))
		}
	}
	return out
}

// defaultAgentItem is the agent the a key opens: config.json's
// defaultAgent when it is enabled, else the first enabled one.
func (a *app) defaultAgentItem() (agentItem, bool) {
	list := a.enabledAgents()
	if len(list) == 0 {
		return agentItem{}, false
	}
	for _, it := range list {
		if it.ID == a.cfg.DefaultAgent {
			return it, true
		}
	}
	return list[0], true
}

// detectAgents looks for the installed agents off the main thread; run it
// at startup and from the settings' 刷新. The login shell's PATH is read
// the first time only.
func (a *app) detectAgents() {
	ag := &a.ag
	ag.loading = true
	ag.seq++
	seq, path, read, exists, home := ag.seq, ag.path, ag.pathRead, ag.exists, ag.home
	a.async(func() {
		if !read {
			// A Dock-launched app's PATH is short: ask the login shell.
			out, _ := a.sys.RunShell("", `$SHELL -lc 'printf %s "$PATH"'`)
			lines := strings.Split(strings.TrimSpace(out), "\n")
			path = strings.TrimSpace(lines[len(lines)-1])
			if path == "" {
				path = os.Getenv("PATH")
			}
		}
		found := agents.Detect(agents.SearchDirs(path, home), exists)
		a.post(func() {
			if seq != ag.seq {
				return
			}
			ag.path, ag.pathRead, ag.found, ag.loading = path, true, found, false
			a.syncAgentRows()
		})
	})
}

// syncAgentRows loads the settings page's edit state from the config.
func (a *app) syncAgentRows() {
	ag := &a.ag
	ag.rows = map[string]*agentRow{}
	for _, f := range ag.found {
		r := &agentRow{cmd: a.agentCommand(f.Entry)}
		if !a.agentEnabled(f.Entry.ID) {
			r.seg = 1
		}
		ag.rows[f.Entry.ID] = r
	}
	a.syncAgentDefault()
}

func (a *app) syncAgentDefault() {
	a.ag.def = ""
	if it, ok := a.defaultAgentItem(); ok {
		a.ag.def = it.ID
	}
}

// setAgentEntry changes an agent's config entry and saves config.json.
func (a *app) setAgentEntry(id string, f func(*agentCfg)) {
	c, ok := a.cfg.Agents[id]
	if !ok {
		c = agentCfg{Enabled: true}
	}
	f(&c)
	m := make(map[string]agentCfg, len(a.cfg.Agents)+1)
	for k, v := range a.cfg.Agents {
		m[k] = v
	}
	m[id] = c
	a.cfg.Agents = m
	a.saveConfig()
}

// setAgentEnabled enables or disables an agent.
func (a *app) setAgentEnabled(id string, on bool) {
	a.setAgentEntry(id, func(c *agentCfg) { c.Enabled = on })
	a.syncAgentDefault()
}

// setAgentCommand sets an agent's command; the default command, or "",
// is stored as "" so that a later catalog change reaches it.
func (a *app) setAgentCommand(id, cmd string) {
	cmd = strings.TrimSpace(cmd)
	if e, ok := agents.ByID(id); ok && cmd == agents.DefaultCommand(e) {
		cmd = ""
	}
	a.setAgentEntry(id, func(c *agentCfg) { c.Command = cmd })
}

// resetAgentCommand brings back an agent's default command.
func (a *app) resetAgentCommand(id string) {
	a.setAgentCommand(id, "")
	if e, ok := agents.ByID(id); ok && a.ag.rows[id] != nil {
		a.ag.rows[id].cmd = agents.DefaultCommand(e)
	}
}

// setDefaultAgent makes id the agent the a key opens.
func (a *app) setDefaultAgent(id string) {
	a.cfg.DefaultAgent = id
	a.ag.def = id
	a.saveConfig()
}

// defaultAgent is the old agentCommand default, kept for migrating it.
const defaultAgent = "claude"

// migrateAgents brings a config.json from before the Agent page along:
// an agentCommand other than claude becomes the command of the agent it
// starts, which becomes the default.
func migrateAgents(cfg *config) {
	cmd := strings.TrimSpace(cfg.AgentCommand)
	if cfg.DefaultAgent != "" || len(cfg.Agents) > 0 || cmd == "" || cmd == defaultAgent {
		return
	}
	bin := agents.FirstWord(cmd)
	for _, e := range agents.Catalog {
		if e.ID == bin || slices.Contains(e.Detect, bin) {
			cfg.Agents = map[string]agentCfg{e.ID: {Enabled: true, Command: cmd}}
			cfg.DefaultAgent = e.ID
			return
		}
	}
}

const agentIcon = 16 // DIPs

// agentGlyph paints an agent's SF Symbol, a letter without them.
func (a *app) agentGlyph(c *ui.Context, id string, tint string) {
	col := tok(c, tint)
	sym := agentSymbol(id)
	ui.Column(c).Width(agentIcon).Height(agentIcon).Draw(func(p *ui.Painter, r ui.Rect) {
		if !sfsymbols.Available() {
			return
		}
		n := int(agentIcon*max(p.Scale(), 1) + 0.5)
		if src, ok := sfsymbols.Image(sym, n, col, sfsymbols.Options{Natural: true}); ok {
			p.Image(src, r, ui.Contain)
		}
	})
}

// agentsPage is the settings' Agent page. Every change is written to
// config.json at once.
func (a *app) agentsPage(c *ui.Context) {
	ag := &a.ag
	ui.Row(c).Gap(px("space-2")).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "已安装 · "+strconv.Itoa(len(ag.found))+" 个").Bold().Grow(1)
		if ui.Button(c.Key("ag.refresh"), "刷新").Disabled(ag.loading).Tooltip("重新检测已安装的 Agent").Clicked() {
			a.detectAgents()
		}
	})
	switch {
	case ag.loading && len(ag.found) == 0:
		uiCaption(c, "正在检测…", "muted")
	case len(ag.found) == 0:
		uiCaption(c, "没有检测到已安装的 Agent", "muted")
	}
	ui.Scroll(c).Height(380).Children(func() {
		ui.Column(c).Gap(px("space-3")).Padding(0, px("space-1.5"), 0, 0).Children(func() {
			for _, f := range ag.found {
				e, row := f.Entry, ag.rows[f.Entry.ID]
				if row == nil {
					continue
				}
				on := row.seg == 0
				isDef := on && ag.def == e.ID
				box := ui.Column(c.Key("ag:" + e.ID)).Gap(px("space-1")).Padding(px("space-1.5")).Radius(px("radius-row"))
				if isDef {
					box.Background(tok(c, "signal-soft"))
				}
				box.Children(func() {
					ui.Row(c).Gap(px("space-2")).AlignItems(ui.Center).Children(func() {
						ui.Row(c).Gap(px("space-2")).AlignItems(ui.Center).Grow(1).Children(func() {
							glyph := "ink"
							if isDef {
								glyph = "signal-text" // the one cyan glyph of the page
							}
							a.agentGlyph(c, e.ID, glyph)
							ui.Text(c, e.Name).Bold()
						})
						if segmented(c.Key("ag.on"), &row.seg, e.Name+" 启用状态", "启用", "禁用") >= 0 {
							a.setAgentEnabled(e.ID, row.seg == 0)
						}
						defLabel := "ink"
						if isDef {
							defLabel = "signal-text"
						}
						if ui.Radio(c.Key("ag.def"), &ag.def, e.ID, "默认").TextColor(tok(c, defLabel)).Disabled(!on).Changed() {
							a.setDefaultAgent(e.ID)
						}
					})
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						if ui.TextInput(c.Key("ag.cmd"), &row.cmd).Label(e.Name + " 命令").Placeholder(agents.DefaultCommand(e)).Grow(1).Changed() {
							a.setAgentCommand(e.ID, row.cmd)
						}
						if ui.Button(c.Key("ag.reset"), "恢复默认").Disabled(strings.TrimSpace(row.cmd) == agents.DefaultCommand(e)).Clicked() {
							a.resetAgentCommand(e.ID)
						}
					})
				})
			}
		})
	})
	uiCaption(c, "命令在终端里运行，默认带 YOLO 参数（跳过权限确认）；可以改成自己想要的参数。检测只在启动和点刷新时进行", "muted")
	ui.Row(c).Justify(ui.End).Children(func() {
		if ui.PrimaryButton(c.Key("agdone"), "完成").Clicked() {
			a.dlgOpen = false
		}
	})
}
