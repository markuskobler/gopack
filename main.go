package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	Blue     = uint8(94)
	Green    = uint8(92)
	Red      = uint8(31)
	Gray     = uint8(90)
	EndColor = "\033[0m"
)

const (
	GopackVersion  = "DEV"
	GopackDir      = ".gopack"
	GopackChecksum = ".gopack/checksum"
)

var (
	pwd        string
	VendorDir  = ".gopack/vendor"
	showColors = false
)

func main() {
	if os.Getenv("GOPACK_COLORS") == "1" {
		showColors = true
	}

	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("gopack version %s\n", GopackVersion)
		os.Exit(0)
	}

	// localize GOPATH
	setupEnv()

	p, err := AnalyzeSourceTree(".")
	if err != nil {
		fail(err)
	}

	config, deps := loadDependencies(".", p)

	if deps == nil {
		fail("Error loading dependency info")
	}

	action := ""
	if len(os.Args) > 1 {
		action = os.Args[1]
	}

	switch action {
	case "dependencytree":
		deps.PrintDependencyTree()
		os.Exit(0)
	case "stats":
		p.PrintSummary()
		os.Exit(0)
	case "installdeps":
		deps.Install(config.Repository)
		os.Exit(0)
	default:
		// fallback to default go command with updated path
		runGo(os.Args[1:]...)
	}

}

func loadDependencies(root string, p *ProjectStats) (*Config, *Dependencies) {
	config, dependencies := loadConfiguration(root)
	if dependencies != nil {
		failWith(dependencies.Validate(p))
		// prepare dependencies
		loadTransitiveDependencies(dependencies)
		config.WriteChecksum()
	}
	return config, dependencies
}

func loadConfiguration(dir string) (*Config, *Dependencies) {
	importGraph := NewGraph()
	config := NewConfig(dir)
	config.InitRepo(importGraph)

	dependencies, err := config.LoadDependencyModel(importGraph)
	if err != nil {
		failf(err.Error())
	}
	return config, dependencies
}

func runGo(args ...string) {
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fail(err)
	}
}

func loadTransitiveDependencies(dependencies *Dependencies) {
	dependencies.VisitDeps(
		func(dep *Dep) {
			fmtcolor(Gray, "     Updating: `%s`\n", dep.Import)
			dep.Get()

			if dep.CheckoutType() != "" {
				fmtcolor(Gray, "      Updated: `%s` at %s %s\n", dep.Import, dep.CheckoutType(), dep.CheckoutSpec)
				dep.switchToBranchOrTag()
			}

			if dep.fetch {
				transitive, err := dep.LoadTransitiveDeps(dependencies.ImportGraph)
				if err != nil {
					failf(err.Error())
				}
				if transitive != nil {
					loadTransitiveDependencies(transitive)
				}
			}
		})
}

// Set the working directory.
// It's the current directory by default.
// It can be overriden setting the environment variable GOPACK_APP_CONFIG.
func setPwd() {
	var dir string
	var err error

	dir = os.Getenv("GOPACK_APP_CONFIG")
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			fail(err)
		}
	}

	pwd = dir
}

// set GOPATH to the local vendor dir
func setupEnv() {
	setPwd()

	if goPath := os.Getenv("GOPATH"); goPath != "" {
		s := filepath.SplitList(goPath)
		dir, err := filepath.Rel(pwd, s[0])
		if err == nil {
			VendorDir = dir
			return
		}
	}

	err := os.Setenv("GOPATH", filepath.Join(pwd, VendorDir))
	if err != nil {
		fail(err)
	}
}

func fmtcolor(c uint8, s string, args ...interface{}) {
	if showColors {
		fmt.Printf("\033[%dm", c)
	}

	if len(args) > 0 {
		fmt.Printf(s, args...)
	} else {
		fmt.Printf(s)
	}

	if showColors {
		fmt.Printf(EndColor)
	}
}

func logcolor(c uint8, s string, args ...interface{}) {
	log.Printf("\033[%dm", c)
	if len(args) > 0 {
		log.Printf(s, args...)
	} else {
		log.Printf(s)
	}
	log.Printf(EndColor)
}

func failf(s string, args ...interface{}) {
	fmtcolor(Red, s, args...)
	log.Println("")
	os.Exit(1)
}

func fail(a ...interface{}) {
	fmt.Printf("\033[%dm", Red)
	fmt.Print(a)
	fmt.Printf(EndColor)
	fmt.Println("")
	os.Exit(1)
}

func failWith(errors []*ProjectError) {
	if len(errors) > 0 {
		fmt.Printf("\033[%dm", Red)
		for _, e := range errors {
			fmt.Printf(e.String())
		}
		fmt.Printf(EndColor)
		fmt.Println()
		os.Exit(len(errors))
	}
}
