//go:build windows
// +build windows

package main

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/andygrunwald/vdf"
	"github.com/briandowns/spinner"
	"github.com/cqroot/prompt"
	"github.com/fatih/color"
	"github.com/google/go-github/v57/github"
)


func HandleInputError(err error) {
	if err != nil {
		if errors.Is(err, prompt.ErrUserQuit) {
			// fmt.Fprintln(os.Stderr, "Error: ", err)
			os.Exit(1)
		} else {
			panic(err)
		}
	}
}

func extractZip(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(dst, os.ModePerm); err != nil {
		return err
	}

	for _, f := range r.File {
		epath := path.Join(dst, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(epath, f.Mode())
			continue
		}

		efile, err := os.Create(epath)
		if err != nil {
			return err
		}
		defer efile.Close()

		freader, err := f.Open()
		if err != nil {
			return err
		}
		defer freader.Close()

		if _, err := io.Copy(efile, freader); err != nil {
			return err
		}
	}
	return nil
}

func downloadFile(url, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func copyFolder(src, dst string) error {
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		if err := os.MkdirAll(dst, 0755); err != nil {
			return err
		}
	}

	files, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, file := range files {
		srcPath := path.Join(src, file.Name())
		dstPath := path.Join(dst, file.Name())

		if file.IsDir() {
			if err := copyFolder(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			srcFile, err := os.Open(srcPath)
			if err != nil {
				return err
			}

			dstFile, err := os.Create(dstPath)
			if err != nil {
				return err
			}
			defer dstFile.Close()

			if _, err := io.Copy(dstFile, srcFile); err != nil {
				return err
			}
			srcFile.Close()
		}
	}
	return nil
}

func copyFile (src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}

func main() {
	fmt.Println("welcome to pawdew updater by embed")
	fmt.Print("https://github.com/embedvr/PawdewUpdater\n\n")
	steamDefault := false
	_, err := os.Stat("C:\\Program Files (x86)\\Steam\\steamapps\\libraryfolders.vdf")
	if err == nil {
		steamDefault = true
	}

	potentialStardewRoute := ""
	stardewRoute := ""

	if !steamDefault {
		log.Println("failed to locate steam folder")
	} else {
		libraryFile, err := os.Open("C:\\Program Files (x86)\\Steam\\steamapps\\libraryfolders.vdf")
		if err != nil {
			fmt.Println("failed to open library file")
		}
		defer libraryFile.Close()

		parser := vdf.NewParser(libraryFile)
		data, err := parser.Parse()
		if err != nil {
			fmt.Println("failed to parse library file")
		}

		libraries := data["libraryfolders"].(map[string]interface {})
		for i := range libraries {
			apps := libraries[i].(map[string]interface {})["apps"].(map[string]interface {})
			
			_, ok := apps["413150"]
			if ok {
				potentialStardewRoute = libraries[i].(map[string]interface {})["path"].(string) + "\\steamapps\\common\\Stardew Valley"
			}
		}
	}

	if potentialStardewRoute != "" {
		stardewRoute, err = prompt.New().Ask("Is this where Stardew is installed?").Choose([]string{potentialStardewRoute, "No"})
		HandleInputError(err)
	}

	if potentialStardewRoute == "" || stardewRoute == "No" {
		stardewRoute, err = prompt.New().Ask("Where is Stardew installed?").Input("")
		HandleInputError(err)
	}

	fmt.Println("checking if valid...")

	validStardewPath := false
	smapiInstalled := false

	if _, err := os.Stat(stardewRoute + "\\Stardew Valley.dll"); err == nil {
		validStardewPath = true
	}

	if validStardewPath {
		fmt.Println("stardew is valid.")
	}

	if _, err := os.Stat(stardewRoute + "\\StardewModdingAPI.dll"); err == nil {
		smapiInstalled = true
	}
	var installSmapi bool

	if smapiInstalled {
		fmt.Println("smapi is installed.")
		installSmapi = false
	}

	if !validStardewPath {
		fmt.Println("Your Stardew Valley installation doesnt seem valid.")
		os.Exit(1)
	}
	if !smapiInstalled {
		smapiOption, err := prompt.New().Ask("Looks like the Stardew Modding API isn't installed. Do you want to install it? (https://github.com/Pathoschild/SMAPI)").Choose([]string{"Yes", "No"})
		HandleInputError(err)

		if smapiOption == "No" {
			os.Exit(1)
		} else {
			installSmapi = true
		}
	}

	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Start()
	defer s.Stop()

	s.Suffix = " cleaning up temporary directory"

	workingDirectory, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	tempPath := path.Join(workingDirectory, "pawdew_temp")
	err = os.RemoveAll(tempPath)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		panic(err)
	}

	os.MkdirAll(tempPath, os.ModePerm)

	if installSmapi {
		s.Suffix = " downloading smapi"

		smapiDownloadURL := ""
		smapiVersion := ""

		client := github.NewClient(nil)
		release, _, err := client.Repositories.GetLatestRelease(context.Background(), "Pathoschild", "SMAPI")
		if err != nil {
			panic(err)
		}

		for _, asset := range release.Assets {
			if asset.GetName() == "SMAPI-" + *release.Name + "-installer.zip" {
				smapiDownloadURL = *asset.BrowserDownloadURL
				smapiVersion = *release.Name
			}
		}

		s.Suffix = " downloading smapi " + smapiVersion + " (https://github.com/Pathoschild/SMAPI)"

		smapiPath := path.Join(tempPath, "smapi.zip")

		err = downloadFile(smapiDownloadURL, smapiPath)
		if err != nil {
			panic(err)
		}

		s.Suffix = " extracting smapi"

		tempExtractPath := path.Join(tempPath, "smapi_extract")

		err = extractZip(smapiPath, tempExtractPath)
		if err != nil {
			panic(err)
		}
		
		smapiTempPath := path.Join(tempExtractPath, "SMAPI " + smapiVersion + " installer", "internal", "windows")

		smapiInstall := path.Join(smapiTempPath, "install.dat")

		smapiInstallOut := path.Join(tempPath, "smapi_install")

		err = extractZip(smapiInstall, smapiInstallOut)
		if err != nil {
			panic(err)
		}

		s.Suffix = " installing smapi"

		err = copyFolder(smapiInstallOut, stardewRoute)
		if err != nil {
			panic(err)
		}

		depsOriginal := path.Join(stardewRoute, "Stardew Valley.deps.json")
		depsSMAPI := path.Join(stardewRoute, "StardewModdingAPI.deps.json")

		err = copyFile(depsOriginal, depsSMAPI)
		if err != nil {
			panic(err)
		}
	}

	url := "https://yumi.helium.ws/modpacks/pawdew.zip"
	s.Suffix = " downloading modpack (" + url + ")"

	zipPath := path.Join(tempPath, "modpack.zip")

	err = downloadFile(url, zipPath)
	if err != nil {
		panic(err)
	}

	s.Suffix = " extracting modpack"

	tempExtractPath := path.Join(tempPath, "extracted")

	err = extractZip(zipPath, tempExtractPath)
	if err != nil {
		panic(err)
	}

	s.Suffix = " clearing currently installed mods"

	if err := os.RemoveAll(path.Join(stardewRoute, "Mods")); err != nil {
		panic(err)
	}

	if err := os.MkdirAll(path.Join(stardewRoute, "Mods"), os.ModePerm); err != nil {
		panic(err)
	}

	s.Suffix = " installing mods"

	err = copyFolder(tempExtractPath, path.Join(stardewRoute, "Mods"))
	if err != nil {
		panic(err)
	}

	s.Suffix = " cleaning up temporary directory"

	err = os.RemoveAll(tempPath)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		panic(err)
	}

	s.Stop()

	fmt.Println("done.")

	if installSmapi {
		color.Set(color.FgHiRed)
		fmt.Print("\n\n")
		fmt.Println("SMAPI was installed, but you need to make sure to add the following line to your launch options")
		fmt.Println("\"" + path.Join(stardewRoute, "StardewModdngAPI.exe") + "\" %command%")
		fmt.Println("You may follow this guide if you need help: https://imgur.com/a/JkDpfKK")
		fmt.Print("\n")
		color.Unset()
	}

	fmt.Print("press enter to exit...")
	fmt.Scanln()
}