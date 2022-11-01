package settings

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/satisfactorymodding/SatisfactoryModManager/utils"
	"github.com/spf13/viper"
)

type SavedModFilters struct {
	Order  string `json:"order"`
	Filter string `json:"filter"`
}

type View string

var (
	VIEW_COMPACT  View = "compact"
	VIEW_EXPANDED View = "expanded"
)

type SettingsData struct {
	FavouriteMods    []string          `json:"favouriteMods"`
	ModFilters       SavedModFilters   `json:"modFilters"`
	AppHeight        int               `json:"appHeight"`
	ExpandedAppWidth int               `json:"expandedAppWidth"`
	StartView        View              `json:"startView"`
	QueueAutoStart   bool              `json:"queueAutoStart"`
	SelectedInstall  string            `json:"selectedInstall"`
	SelectedProfile  map[string]string `json:"selectedProfile"`
	Konami           bool              `json:"konami"`
	LaunchButton     string            `json:"launchButton"`
}

var Settings SettingsData
var settingsFileName = "settings.json"

func LoadSettings() error {
	settingsFilePath := filepath.Join(viper.GetString("local-dir"), settingsFileName)

	_, err := os.Stat(settingsFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrap(err, "failed to stat settings file")
		}

		Settings = SettingsData{
			FavouriteMods:    []string{},
			ModFilters:       SavedModFilters{Order: "Last updated", Filter: "Compatible"},
			AppHeight:        utils.UnexpandedMinHeight,
			ExpandedAppWidth: utils.UnexpandedMinWidth,
			StartView:        VIEW_COMPACT,
			SelectedInstall:  "",
			SelectedProfile:  map[string]string{},
		}
		err = SaveSettings()
		if err != nil {
			return errors.Wrap(err, "failed to save default settings")
		}
	}

	setDefaults()

	settingsFile, err := os.ReadFile(filepath.Join(viper.GetString("local-dir"), settingsFileName))
	if err != nil {
		return errors.Wrap(err, "failed to read settings")
	}

	if err := json.Unmarshal(settingsFile, &Settings); err != nil {
		return errors.Wrap(err, "failed to unmarshal settings")
	}

	return nil
}

func setDefaults() {
	Settings.FavouriteMods = []string{}
	Settings.ModFilters.Order = "Last updated"
	Settings.ModFilters.Filter = "Compatible"
	Settings.AppHeight = utils.UnexpandedMinHeight
	Settings.ExpandedAppWidth = utils.UnexpandedMinWidth
	Settings.StartView = VIEW_COMPACT
	Settings.QueueAutoStart = true
	Settings.SelectedProfile = map[string]string{}
	Settings.LaunchButton = "normal"
}

func SaveSettings() error {
	settingsFile, err := json.MarshalIndent(Settings, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal settings")
	}
	err = os.WriteFile(filepath.Join(viper.GetString("local-dir"), settingsFileName), settingsFile, 0755)
	if err != nil {
		return errors.Wrap(err, "failed to write settings")
	}

	return nil
}
