package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/liliang-cn/gosible/pkg/utils"
)

func main() {
	var (
		useCase = flag.String("use-case", "ansible-like", "Use case: web, archive, build, ansible-like")
		install = flag.Bool("install", false, "Attempt to install missing dependencies")
		sudo    = flag.Bool("sudo", false, "Use sudo for installation")
	)
	flag.Parse()

	fmt.Println("Gosible Prerequisites Checker")
	fmt.Println("==============================")
	fmt.Println()

	// Create a command checker
	checker := utils.NewCommandChecker()

	// Check common tools individually
	fmt.Println("Checking Common Tools:")
	commonTools := []string{"curl", "wget", "tar", "zip", "unzip", "git", "ssh", "rsync"}
	
	for _, tool := range commonTools {
		if checker.CommandAvailable(tool) {
			version, _ := checker.GetVersion(tool)
			// Truncate version to first line
			if len(version) > 50 {
				version = version[:47] + "..."
			}
			fmt.Printf("  ✓ %-10s (found)\n", tool)
		} else {
			fmt.Printf("  ✗ %-10s (missing)\n", tool)
		}
	}

	fmt.Println()

	// Get dependencies for the specified use case
	deps := utils.GetCommonDependencies(*useCase)
	fmt.Printf("Checking Dependencies for '%s' use case:\n", *useCase)
	fmt.Println("==========================================")

	// Check dependencies
	report, err := checker.CheckDependencies(deps)
	if err != nil {
		log.Fatalf("Error checking dependencies: %v", err)
	}

	// Print the report
	fmt.Println(report.String())

	// If there are missing required dependencies
	if !report.AllRequiredPresent {
		fmt.Println("\n⚠️  Missing required dependencies detected!")
		
		if *install {
			fmt.Println("\nAttempting to install missing dependencies...")
			ctx := context.Background()
			
			for cmd := range report.Missing {
				fmt.Printf("Installing %s...\n", cmd)
				if err := checker.InstallCommand(ctx, cmd, *sudo); err != nil {
					fmt.Printf("  Failed to install %s: %v\n", cmd, err)
				} else {
					fmt.Printf("  ✓ %s installed successfully\n", cmd)
				}
			}
		} else {
			fmt.Println("\nTo install missing dependencies, run with -install flag")
			fmt.Println("Example: go run main.go -install -sudo")
		}
		
		os.Exit(1)
	}

	fmt.Println("\n✅ All required dependencies are present!")
	
	// Example: Ensure specific commands for different operations
	fmt.Println("\nOperation-Specific Checks:")
	
	// For downloading files
	fmt.Println("\nDownloading files:")
	if err := checker.EnsureCommand("curl"); err != nil {
		if err := checker.EnsureCommand("wget"); err != nil {
			fmt.Printf("  ⚠️  Neither curl nor wget available: %v\n", err)
		} else {
			fmt.Println("  ✓ wget is available for downloads")
		}
	} else {
		fmt.Println("  ✓ curl is available for downloads")
	}
	
	// For archive operations
	fmt.Println("\nArchive operations:")
	if err := checker.EnsureCommand("tar"); err != nil {
		fmt.Printf("  ⚠️  tar not available: %v\n", err)
	} else {
		fmt.Println("  ✓ tar is available for archive operations")
	}
	
	if err := checker.EnsureCommand("zip"); err != nil {
		fmt.Printf("  ⚠️  zip not available: %v\n", err)
	} else {
		fmt.Println("  ✓ zip is available")
	}
	
	if err := checker.EnsureCommand("unzip"); err != nil {
		fmt.Printf("  ⚠️  unzip not available: %v\n", err)
	} else {
		fmt.Println("  ✓ unzip is available")
	}
}