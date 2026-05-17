package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janbaraniewski/synology-ctl/internal/dsm"
	"github.com/janbaraniewski/synology-ctl/internal/tui"
)

func renderShareDetail(t tui.Theme, width, height int, s dsm.Share) string {
	_ = height
	if width < 60 {
		width = 60
	}
	parts := []string{
		hero(t, width, "▦", s.Name, "", s.Path),
	}
	if s.ShareQuota > 0 {
		usedMB := uint64(s.ShareQuotaUsed) * 1024 * 1024
		totalMB := uint64(s.ShareQuota) * 1024 * 1024
		ratio := 0.0
		if totalMB > 0 {
			ratio = float64(usedMB) / float64(totalMB)
		}
		parts = append(parts, gaugeCard(t, width, " Quota usage ",
			fmt.Sprintf("%5.1f%%", ratio*100), ratio,
			fmt.Sprintf("%s used · %s total", HumanBytes(usedMB), HumanBytes(totalMB))))
	} else {
		parts = append(parts, t.Card(false).Width(width-2).Render(
			t.Title().Render(" Quota ")+"\n"+
				lipgloss.NewStyle().Foreground(t.Muted).Render("  No quota set")))
	}

	chip := func(label string, on bool) string {
		st := t.HealthStyle("disabled")
		if on {
			st = t.HealthStyle("enabled")
		}
		return st.Render(" " + label + " ")
	}
	chips := []string{
		chip("encrypted", s.IsEncrypted()),
		chip("recycle bin", s.EnableRecycle),
		chip("hidden", s.Hidden),
		chip("read-only", s.Readonly),
		chip("USB share", s.IsUsbShare),
		chip("sync share", s.IsSyncShare),
		chip("cloud sync", s.IsCloudSync),
		chip("admin recycle", bool(s.RecycleAdminOnly)),
	}
	parts = append(parts, chipsCard(t, width, " Flags ", chips))

	parts = append(parts, propsCard(t, width, " Properties ", [][2]string{
		{"Name", s.Name},
		{"Path", s.Path},
		{"Description", s.Desc},
		{"UUID", s.UUID},
		{"Encryption status", encryptionString(s.Encryption, s.EncStatus)},
		{"Quota (MB)", fmt.Sprintf("%d", s.ShareQuota)},
		{"Quota used (MB)", fmt.Sprintf("%d", s.ShareQuotaUsed)},
	}))

	parts = append(parts, noteCard(t, width,
		"  esc to go back · share permissions/ACLs are read-only here for safety"))
	return strings.Join(parts, "\n")
}

func encryptionString(enc, encStatus int) string {
	if enc == 0 && encStatus == 0 {
		return "none"
	}
	return fmt.Sprintf("enc=%d, status=%d", enc, encStatus)
}
