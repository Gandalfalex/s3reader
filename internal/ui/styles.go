package ui

import tk "github.com/Gandalfalex/tuikit"

var (
	theme = tk.TokyoNight

	cAccent, cMuted, cDir, cErr, cOK, cWarn = theme.C.Accent, theme.C.Muted, theme.C.Dir, theme.C.Err, theme.C.OK, theme.C.Warn

	sTitle, sCrumb, sMuted, sDir, sSel = theme.Title, theme.Crumb, theme.Muted, theme.Dir, theme.Sel
	sHeader, sErr, sOK, sWarn, sBar    = theme.Header, theme.ErrText, theme.OKText, theme.Warn, theme.Bar
	sMode, sSection                    = theme.Mode, theme.Section
	sLabel                             = theme.Crumb.Width(18)
)
