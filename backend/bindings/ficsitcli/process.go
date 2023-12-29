package ficsitcli

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/satisfactorymodding/ficsit-cli/cli"
	ficsitUtils "github.com/satisfactorymodding/ficsit-cli/utils"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/satisfactorymodding/SatisfactoryModManager/backend/utils"
)

func (f *FicsitCLI) validateInstall(installation *InstallationInfo, progressItem string) error {
	f.EmitModsChange()
	defer f.EmitModsChange()

	installChannel := make(chan cli.InstallUpdate)

	defer f.setProgress(f.progress)

	type modProgress struct {
		downloadProgress ficsitUtils.GenericProgress
		extractProgress  ficsitUtils.GenericProgress
		downloading      bool
		complete         bool
	}
	modProgresses := xsync.NewMapOf[string, modProgress]()

	progressTicker := time.NewTicker(100 * time.Millisecond)
	done := make(chan bool)
	defer progressTicker.Stop()
	defer close(done)

	downloadProgressTracker := utils.NewProgressTracker(time.Second * 5)
	extractProgressTracker := utils.NewProgressTracker(time.Second * 5)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-progressTicker.C:
				downloadBytesProgress := ficsitUtils.GenericProgress{}
				extractBytesProgress := ficsitUtils.GenericProgress{}
				downloadModsProgress := ficsitUtils.GenericProgress{}
				extractModsProgress := ficsitUtils.GenericProgress{}

				hasDownloading := false

				modProgresses.Range(func(key string, value modProgress) bool {
					if value.downloadProgress.Total != 0 {
						downloadModsProgress.Total++
						if value.complete || !value.downloading {
							downloadModsProgress.Completed++
						}
						if !value.complete && value.downloading {
							hasDownloading = true
						}
					}
					// Extraction progress is not available while the mod is still being downloaded,
					// but we should still count it as an extraction that has to execute.
					if value.downloadProgress.Total != 0 || value.extractProgress.Total != 0 {
						extractModsProgress.Total++
						if value.complete {
							extractModsProgress.Completed++
						}
					}

					downloadBytesProgress.Completed += value.downloadProgress.Completed
					downloadBytesProgress.Total += value.downloadProgress.Total
					extractBytesProgress.Completed += value.extractProgress.Completed
					extractBytesProgress.Total += value.extractProgress.Total

					return true
				})

				downloadProgressTracker.Add(downloadBytesProgress.Completed)
				downloadProgressTracker.Total = downloadBytesProgress.Total
				extractProgressTracker.Add(extractBytesProgress.Completed)
				extractProgressTracker.Total = extractBytesProgress.Total

				if hasDownloading {
					if downloadBytesProgress.Total != 0 {
						eta := downloadProgressTracker.ETA().Round(time.Second)
						etaText := eta.String()
						if eta == 0 {
							etaText = "soon™"
						}
						f.setProgress(&Progress{
							Item: progressItem,
							Message: fmt.Sprintf(
								"Downloading %d/%d mods: %s/%s, %s/s, %s",
								downloadModsProgress.Completed, downloadModsProgress.Total,
								humanize.Bytes(uint64(downloadBytesProgress.Completed)), humanize.Bytes(uint64(downloadBytesProgress.Total)),
								humanize.Bytes(uint64(downloadProgressTracker.Speed())),
								etaText,
							),
							Progress: downloadBytesProgress.Percentage(),
						})
					}
				} else {
					if extractBytesProgress.Total != 0 {
						eta := extractProgressTracker.ETA().Round(time.Second)
						etaText := eta.String()
						if eta == 0 {
							etaText = "soon™"
						}
						f.setProgress(&Progress{
							Item: progressItem,
							Message: fmt.Sprintf(
								"Extracting %d/%d mods: %s/%s, %s/s, %s",
								extractModsProgress.Completed, extractModsProgress.Total,
								humanize.Bytes(uint64(extractBytesProgress.Completed)), humanize.Bytes(uint64(extractBytesProgress.Total)),
								humanize.Bytes(uint64(extractProgressTracker.Speed())),
								etaText,
							),
							Progress: extractBytesProgress.Percentage(),
						})
					}
				}
			}
		}
	}()

	go func() {
		for update := range installChannel {
			if update.Type == cli.InstallUpdateTypeOverall {
				// Although this wouldn't cause any issues in the progress generation above, we can ignore this update.
				continue
			}
			modProgresses.Compute(update.Item.Mod, func(oldValue modProgress, loaded bool) (modProgress, bool) {
				if oldValue.complete {
					// Sometimes extract updates are received after the mod is marked as complete.
					return oldValue, false
				}
				oldValue.complete = update.Type == cli.InstallUpdateTypeModComplete
				oldValue.downloading = update.Type == cli.InstallUpdateTypeModDownload

				switch update.Type {
				case cli.InstallUpdateTypeModDownload:
					oldValue.downloadProgress = update.Progress
				case cli.InstallUpdateTypeModExtract:
					oldValue.extractProgress = update.Progress
				}
				return oldValue, false
			})
		}
	}()

	installErr := installation.Installation.Install(f.ficsitCli, installChannel)
	if installErr != nil {
		return errors.Wrap(installErr, "Failed to install")
	}
	return nil
}

func (f *FicsitCLI) EmitModsChange() {
	lockfileMods, err := f.GetSelectedInstallLockfileMods()
	if err != nil {
		slog.Error("failed to load lockfile", slog.Any("error", err))
		return
	}
	wailsRuntime.EventsEmit(f.ctx, "lockfileMods", lockfileMods)
	wailsRuntime.EventsEmit(f.ctx, "manifestMods", f.GetSelectedInstallProfileMods())
}

func (f *FicsitCLI) EmitGlobals() {
	if f.ctx == nil {
		// This function can be called from AddRemoteServer, which is used during initialization
		// at which point the context is not set yet.
		// We can safely ignore this call.
		return
	}
	wailsRuntime.EventsEmit(f.ctx, "installations", f.GetInstallationsInfo())
	wailsRuntime.EventsEmit(f.ctx, "remoteServers", f.GetRemoteInstallations())
	profileNames := make([]string, 0, len(f.ficsitCli.Profiles.Profiles))
	for k := range f.ficsitCli.Profiles.Profiles {
		profileNames = append(profileNames, k)
	}
	wailsRuntime.EventsEmit(f.ctx, "profiles", profileNames)

	if f.selectedInstallation == nil {
		return
	}

	wailsRuntime.EventsEmit(f.ctx, "selectedInstallation", f.selectedInstallation.Installation.Path)
	wailsRuntime.EventsEmit(f.ctx, "selectedProfile", f.selectedInstallation.Installation.Profile)
	wailsRuntime.EventsEmit(f.ctx, "modsEnabled", !f.selectedInstallation.Installation.Vanilla)
}

func (f *FicsitCLI) InstallMod(mod string) error {
	if f.progress != nil {
		return errors.New("Another operation in progress")
	}

	if f.selectedInstallation == nil {
		return errors.New("No installation selected")
	}

	profileName := f.selectedInstallation.Installation.Profile
	profile := f.GetProfile(profileName)

	profileErr := profile.AddMod(mod, ">=0.0.0")
	if profileErr != nil {
		return errors.Wrapf(profileErr, "Failed to add mod: %s@latest", mod)
	}

	f.progress = &Progress{
		Item:     mod,
		Message:  "Finding the best version to install",
		Progress: -1,
	}

	f.setProgress(f.progress)

	defer f.setProgress(nil)

	installErr := f.validateInstall(f.selectedInstallation, mod)

	if installErr != nil {
		return errors.Wrapf(installErr, "Failed to install mod: %s@latest", mod)
	}

	_ = f.ficsitCli.Profiles.Save()

	return nil
}

func (f *FicsitCLI) InstallModVersion(mod string, version string) error {
	if f.progress != nil {
		return errors.New("Another operation in progress")
	}

	if f.selectedInstallation == nil {
		return errors.New("No installation selected")
	}

	profileName := f.selectedInstallation.Installation.Profile
	profile := f.GetProfile(profileName)

	profileErr := profile.AddMod(mod, version)
	if profileErr != nil {
		return errors.Wrapf(profileErr, "Failed to add mod: %s@%s", mod, version)
	}

	f.progress = &Progress{
		Item:     mod,
		Message:  "Finding the best version to install",
		Progress: -1,
	}

	f.setProgress(f.progress)

	defer f.setProgress(nil)

	installErr := f.validateInstall(f.selectedInstallation, mod)

	if installErr != nil {
		return errors.Wrapf(installErr, "Failed to install mod: %s@%s", mod, version)
	}

	_ = f.ficsitCli.Profiles.Save()

	return nil
}

func (f *FicsitCLI) RemoveMod(mod string) error {
	if f.progress != nil {
		return errors.New("Another operation in progress")
	}

	if f.selectedInstallation == nil {
		return errors.New("No installation selected")
	}

	profileName := f.selectedInstallation.Installation.Profile
	profile := f.GetProfile(profileName)

	profile.RemoveMod(mod)

	f.progress = &Progress{
		Item:     mod,
		Message:  "Checking for mods that are no longer needed",
		Progress: -1,
	}

	f.setProgress(f.progress)

	defer f.setProgress(nil)

	installErr := f.validateInstall(f.selectedInstallation, mod)

	if installErr != nil {
		return errors.Wrapf(installErr, "Failed to remove mod: %s", mod)
	}

	_ = f.ficsitCli.Profiles.Save()

	return nil
}

func (f *FicsitCLI) EnableMod(mod string) error {
	if f.progress != nil {
		return errors.New("Another operation in progress")
	}

	if f.selectedInstallation == nil {
		return errors.New("No installation selected")
	}

	profileName := f.selectedInstallation.Installation.Profile
	profile := f.GetProfile(profileName)

	profile.SetModEnabled(mod, true)

	f.progress = &Progress{
		Item:     mod,
		Message:  "Finding the best version to install",
		Progress: -1,
	}

	f.setProgress(f.progress)

	defer f.setProgress(nil)

	installErr := f.validateInstall(f.selectedInstallation, mod)

	if installErr != nil {
		return errors.Wrapf(installErr, "Failed to enable mod: %s", mod)
	}

	_ = f.ficsitCli.Profiles.Save()

	return nil
}

func (f *FicsitCLI) DisableMod(mod string) error {
	if f.progress != nil {
		return errors.New("Another operation in progress")
	}

	if f.selectedInstallation == nil {
		return errors.New("No installation selected")
	}

	profileName := f.selectedInstallation.Installation.Profile
	profile := f.GetProfile(profileName)

	profile.SetModEnabled(mod, false)

	f.progress = &Progress{
		Item:     mod,
		Message:  "Checking for mods that are no longer needed",
		Progress: -1,
	}

	f.setProgress(f.progress)

	defer f.setProgress(nil)

	installErr := f.validateInstall(f.selectedInstallation, mod)

	if installErr != nil {
		return errors.Wrapf(installErr, "Failed to disable mod: %s", mod)
	}

	_ = f.ficsitCli.Profiles.Save()

	return nil
}
