package ui

import (
	"charm.land/lipgloss/v2"
)

var (
	cAccent  = lipgloss.Color("#7aa2f7")
	cMuted   = lipgloss.Color("#565f89")
	cDir     = lipgloss.Color("#7dcfff")
	cErr     = lipgloss.Color("#f7768e")
	cOK      = lipgloss.Color("#9ece6a")
	cWarn    = lipgloss.Color("#e0af68")
	cSelBg   = lipgloss.Color("#283457")
	cBarBg   = lipgloss.Color("#1f2335")
	sTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#1a1b26")).Background(cAccent).Padding(0, 1)
	sCrumb   = lipgloss.NewStyle().Foreground(cAccent)
	sMuted   = lipgloss.NewStyle().Foreground(cMuted)
	sDir     = lipgloss.NewStyle().Foreground(cDir).Bold(true)
	sSel     = lipgloss.NewStyle().Background(cSelBg).Bold(true)
	sHeader  = lipgloss.NewStyle().Foreground(cMuted).Bold(true)
	sErr     = lipgloss.NewStyle().Foreground(cErr)
	sOK      = lipgloss.NewStyle().Foreground(cOK)
	sWarn    = lipgloss.NewStyle().Foreground(cWarn)
	sBar     = lipgloss.NewStyle().Background(cBarBg)
	sMode    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#1a1b26")).Padding(0, 1)
	sLabel   = lipgloss.NewStyle().Foreground(cAccent).Width(18)
	sSection = lipgloss.NewStyle().Foreground(cWarn).Bold(true)
)
