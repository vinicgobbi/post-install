// Package config guarda as listas compartilhadas entre módulos.
package config

// FlatpaksKDE é a lista otimizada para o Plasma (equivalentes em Qt + apps
// mantidos por escolha).
var FlatpaksKDE = []string{
	// Apps mantidos por escolha:
	"com.spotify.Client", "io.ente.auth", "io.github.flattool.Warehouse",
	"md.obsidian.Obsidian", "org.libreoffice.LibreOffice",
	"org.onlyoffice.desktopeditors", "io.dbeaver.DBeaverCommunity",

	// Já são nativos em Qt:
	"com.mikrotik.WinBox", "com.obsproject.Studio", "org.qbittorrent.qBittorrent",
	"org.telegram.desktop", "org.videolan.VLC", "com.rtosta.zapzap",

	// Equivalentes em Qt:
	"org.kde.umbrello", // no lugar do Gaphor
	"org.kde.krdc",     // no lugar do Remmina

	// Sem Flatpak no Flathub (org.kde.krusader não existe); mantido:
	"org.filezillaproject.Filezilla",
}

// FlatpaksGTK é a lista padrão (GNOME/GTK).
var FlatpaksGTK = []string{
	"com.getpostman.Postman", "com.github.tchx84.Flatseal",
	"com.mattjakeman.ExtensionManager",
	"com.obsproject.Studio", "com.spotify.Client", "dev.vencord.Vesktop",
	"io.dbeaver.DBeaverCommunity", "io.ente.auth", "io.github.flattool.Ignition", "io.github.flattool.Warehouse",
	"io.missioncenter.MissionCenter", "md.obsidian.Obsidian",
	"org.filezillaproject.Filezilla", "org.gaphor.Gaphor", "org.gnome.Boxes",
	"org.libreoffice.LibreOffice", "org.onlyoffice.desktopeditors", "org.qbittorrent.qBittorrent",
	"org.remmina.Remmina", "org.telegram.desktop", "org.videolan.VLC", "me.iepure.devtoolbox",
	"com.rtosta.zapzap", "be.alexandervanhee.gradia", "org.gnome.Snapshot",
	"org.gnome.Weather", "net.blip.Blip",
}

var FlatpaksJogos = []string{
	"com.heroicgameslauncher.hgl", "com.valvesoftware.Steam",
	"com.vysp3r.ProtonPlus", "org.prismlauncher.PrismLauncher",
}
