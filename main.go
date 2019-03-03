package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const codeWikiRepo = "git@github.com:logwolvy/logwolvy.github.io.git"

type mode int

const (
	gitWatchMode mode = 0
	manualMode   mode = 1
)

var projectDir string
var tempCodeWikiDir = fmt.Sprintf("/tmp/%s", randomString(8))
var beginTag = regexp.MustCompile(`wiki\/\S*`)
var endTag = regexp.MustCompile(`end-wiki`)
var sideBarData []byte
var executionMode mode

func randomString(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, n)
	rand.Seed(time.Now().UnixNano())
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func setStartupMode() {
	setupDir := flag.String("setup", "", "project directory to watch staged files")
	manualFlag := flag.Bool("manual-mode", false, "Manual Mode")
	flag.Parse()

	// Setup Git watch mode
	// Append to git precommit hook
	if len(*setupDir) > 0 {
		projectDir = *setupDir
		wikiHook := fmt.Sprintf("#!/bin/sh\n wikiator %s\n exit 0\n", projectDir)

		// If the file doesn't exist, create it, or append to the file
		f, err := os.OpenFile(fmt.Sprintf("%s/%s", projectDir, ".git/hooks/pre-commit"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := f.Write([]byte(wikiHook)); err != nil {
			log.Fatal(err)
		}
		if err := f.Close(); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Wikiator successfully linked to", projectDir)
		os.Exit(0)
	}

	if len(os.Args[2]) > 0 && *manualFlag {
		// Set Manual mode (recursive)
		executionMode = manualMode
		projectDir = os.Args[2]
	} else if len(os.Args[1]) > 0 {
		// Set Git watch mode
		executionMode = gitWatchMode
		projectDir = os.Args[1]
	} else {
		panic("Wrong arguments given")
	}
}

func main() {
	setStartupMode()

	// TODO - make this concurrent
	if executionMode == gitWatchMode {
		fetchCodeWiki()
		generateWikis(getModifiedFiles)
	} else {
		fetchCodeWiki()
		processFilesRecursively()
	}
	pushWikiChanges()
}

func fetchCodeWiki() {
	fmt.Println("Pulling code wiki repo in", tempCodeWikiDir)
	cmd := exec.Command("git", "clone", codeWikiRepo, tempCodeWikiDir)
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	sideBarData, _ = ioutil.ReadFile(fmt.Sprintf("%s/_sidebar.md", tempCodeWikiDir))
}

// For Git watch mode
func getModifiedFiles() []string {
	// Get staged file paths
	cmd := exec.Command("git", "-C", projectDir, "diff",
		"--cached", "--name-only", "--diff-filter=AMC",
	)
	stdout, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	// splits & trims leading/trailing newline characters
	filePaths := strings.Split(strings.TrimSpace(fmt.Sprintf("%s", stdout)), "\n")
	sanitizedFilePaths := make([]string, 0)
	for _, filePath := range filePaths {
		sanitizedFilePaths = append(sanitizedFilePaths, strings.TrimSpace(filePath))
	}
	fmt.Println("Checking Staged Files...\n", strings.Join(sanitizedFilePaths, "\n"))
	return sanitizedFilePaths
}

func processFilesRecursively() {
	_ = filepath.Walk(projectDir, func(path string, f os.FileInfo, err error) error {
		processFile(path)
		return nil
	})
	os.Exit(0)
}

func generateWikis(f func() []string) {
	for _, filePath := range f() {
		absPath := fmt.Sprintf("%s/%s", projectDir, filePath)
		processFile(absPath)
	}
}

func processFile(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(file)
	tagMatch := false
	contents := make([]string, 0)
	var beginTagMatch []string
	var endTagMatch []string
	for scanner.Scan() {
		line := scanner.Text()
		beginTagMatch = beginTag.FindStringSubmatch(line)
		endTagMatch = endTag.FindStringSubmatch(line)

		if len(beginTagMatch) > 0 {
			tagMatch = true
		} else if len(endTagMatch) > 0 {
			break
		}

		if tagMatch {
			contents = append(contents, line)
		}
	}

	if len(contents) > 0 {
		createWikiFile(contents)
	}
}

func createWikiFile(contents []string) {
	tag := contents[0]
	fileProps := parseTag(tag)

	rootDir := fmt.Sprintf("%s/pages/%s", tempCodeWikiDir, fileProps.extension)
	os.MkdirAll(rootDir, os.ModePerm)

	var subDir string
	if len(fileProps.module) > 0 {
		subDir = fmt.Sprintf("%s/%s", fileProps.module, fileProps.desc)
	} else {
		subDir = fileProps.desc
	}

	suffix := randomString(3) + ".md"
	fpath := fmt.Sprintf("%s/%s-%s", rootDir, subDir, suffix)
	fmt.Println("Creating entry in", fpath)
	f, err := os.Create(fpath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	wrapper := make([]string, 1)
	heading := strings.Title(strings.Replace(fileProps.desc, "-", " ", -1))
	wrapper[0] = fmt.Sprintf("# %s\n", heading)
	contents[0] = fmt.Sprintf("```%s\n", fileProps.extension)
	contents = append(contents, "```")
	wrapper = append(wrapper, contents...)
	_, err = f.WriteString(strings.Join(wrapper, "\n"))

	f.Sync()

	updateSidebar(fileProps, strings.Replace(fpath, tempCodeWikiDir, "", 1))
}

type position struct {
	extension, module, desc string
}

func parseTag(tag string) position {
	tagParts := strings.Split(tag, "/")[1:]
	fmt.Println("Current file tags", tagParts)
	if len(tagParts) == 3 {
		return position{extension: tagParts[0], module: tagParts[1], desc: tagParts[2]}
	}
	return position{extension: tagParts[0], desc: tagParts[1]}
}

func updateSidebar(props position, relativeURL string) {
	f, err := os.OpenFile(fmt.Sprintf("%s/%s", tempCodeWikiDir, "_sidebar.md"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()

	link := fmt.Sprintf("- [%s](%s)", props.desc, relativeURL)
	if _, err := f.WriteString(link + "\n"); err != nil {
		log.Println(err)
	}
}

func pushWikiChanges() {
	cmd := exec.Command("git", "-C", tempCodeWikiDir, "add", ".")
	cmd.Run()
	cmd = exec.Command("git", "-C", tempCodeWikiDir, "commit", "-m", "test commit")
	cmd.Run()
	cmd = exec.Command("git", "-C", tempCodeWikiDir, "push", "origin", "master")
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Wiki pushed & deployed")
}
