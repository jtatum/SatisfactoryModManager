package ficsitcli

import (
	"log/slog"

	"github.com/pkg/errors"
	"github.com/satisfactorymodding/ficsit-cli/cli"
	resolver "github.com/satisfactorymodding/ficsit-resolver"
	"github.com/spf13/viper"
)

type Update struct {
	Item           string `json:"item"`
	CurrentVersion string `json:"currentVersion"`
	NewVersion     string `json:"newVersion"`
}

func (f *FicsitCLI) CheckForUpdates() ([]Update, error) {
	selectedInstallation := f.GetSelectedInstall()

	if selectedInstallation == nil {
		return []Update{}, nil
	}
	l := slog.With(slog.String("task", "checkForUpdates"))

	currentLockfile, err := selectedInstallation.LockFile(f.ficsitCli)
	if err != nil {
		l.Error("failed to get current lockfile", slog.Any("error", err))
		return nil, errors.Wrap(err, "failed to get current lockfile")
	}

	if currentLockfile == nil {
		return nil, nil
	}

	profile := f.GetProfile(selectedInstallation.Profile)

	res := resolver.NewDependencyResolver(f.ficsitCli.Provider, viper.GetString("api-base"))

	gameVersion, err := selectedInstallation.GetGameVersion(f.ficsitCli)
	if err != nil {
		l.Error("failed to get game version", slog.Any("error", err))
		return nil, errors.Wrap(err, "failed to get game version")
	}

	updateProfile := &cli.Profile{
		Name: "Update temp",
		Mods: make(map[string]cli.ProfileMod),
	}
	for modReference, modData := range profile.Mods {
		updateProfile.Mods[modReference] = cli.ProfileMod{
			Enabled: modData.Enabled,
			Version: ">=0.0.0",
		}
	}
	newLockfile, err := updateProfile.Resolve(res, nil, gameVersion)
	if err != nil {
		l.Error("failed to resolve dependencies", slog.Any("error", err))
		var solvingError resolver.DependencyResolverError
		if errors.As(err, &solvingError) {
			return nil, solvingError
		}
		return nil, err //nolint:wrapcheck
	}

	updates := []Update{}

	for modReference, newLockedMod := range newLockfile.Mods {
		if prevLockedMod, ok := currentLockfile.Mods[modReference]; ok {
			if newLockedMod.Version != prevLockedMod.Version {
				updates = append(updates, Update{
					Item:           modReference,
					CurrentVersion: prevLockedMod.Version,
					NewVersion:     newLockedMod.Version,
				})
			}
		}
	}

	return updates, nil
}

func (f *FicsitCLI) UpdateMods(mods []string) error {
	l := slog.With(slog.String("task", "updateMods"))

	if f.progress != nil {
		l.Error("another operation in progress")
		return errors.New("another operation in progress")
	}

	selectedInstallation := f.GetSelectedInstall()

	if selectedInstallation == nil {
		return errors.New("no installation selected")
	}

	profile := f.GetProfile(selectedInstallation.Profile)
	for _, modReference := range mods {
		if _, ok := profile.Mods[modReference]; !ok {
			l.Warn("mod not found in profile", slog.String("mod", modReference))
			continue
		}
		profile.Mods[modReference] = cli.ProfileMod{
			Enabled: profile.Mods[modReference].Enabled,
			Version: ">=0.0.0",
		}
	}

	err := selectedInstallation.UpdateMods(f.ficsitCli, mods)
	if err != nil {
		l.Error("failed to update mods", slog.Any("error", err))
		var solvingError resolver.DependencyResolverError
		if errors.As(err, &solvingError) {
			return solvingError
		}
		return err //nolint:wrapcheck
	}

	f.progress = &Progress{
		Item:     "__update__",
		Message:  "Updating...",
		Progress: -1,
	}

	f.setProgress(f.progress)

	defer f.setProgress(nil)

	err = f.validateInstall(selectedInstallation, "__update__")

	if err != nil {
		l.Error("failed to validate installation", slog.Any("error", err))
		var solvingError resolver.DependencyResolverError
		if errors.As(err, &solvingError) {
			return solvingError
		}
		return err
	}

	_ = f.ficsitCli.Profiles.Save()

	return nil
}
