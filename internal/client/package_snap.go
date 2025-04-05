package client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/neuspaces/terraform-provider-system/internal/system"
	"sort"
	"strings"
)

const SnapPackageManager PackageManager = "snap"

var (
	ErrSnapPackage = errors.New("snap package resource")

	ErrSnapPackageManagerNotAvailable = errors.Join(ErrSnapPackage, errors.New("snap not available"))

	ErrSnapPackageManager = errors.Join(ErrSnapPackage, errors.New("snap error"))

	ErrSnapPackageUnexpected = errors.Join(ErrSnapPackage, errors.New("unexpected error"))
)

func NewSnapPackageClient(s system.System) PackageClient {
	return &snapPackageClient{
		s: s,
	}
}

type snapPackageClient struct {
	s system.System
}

// Get returns a list of Packages which contain all installed packages. Each Package contains the available version.
func (c *snapPackageClient) Get(ctx context.Context) (Packages, error) {
	cmd := NewCommand(`_do() { which snap >/dev/null 2>&1; which_snap_rc=$?; if [ $which_snap_rc -eq 0 ]; then snap list; else echo "which_snap_rc=${which_snap_rc}"; fi }; _do;`)
	res, err := ExecuteCommand(ctx, c.s, cmd)
	if err != nil {
		return nil, errors.Join(ErrSnapPackage, err)
	}

	if strings.HasPrefix(res.StdoutString(), "which_snap_rc=") && res.StdoutString() != "which_snap_rc=0" {
		return nil, ErrSnapPackageManagerNotAvailable
	}

	if res.ExitCode != 0 {
		return nil, ErrSnapPackageUnexpected
	}

	// Parse output of `snap list`
	// Format is tabular: Name  Version  Rev  Tracking  Publisher  Notes
	scanner := bufio.NewScanner(strings.NewReader(res.StdoutString()))

	// Skip header row
	if !scanner.Scan() {
		return nil, ErrSnapPackageUnexpected
	}

	// Construct result
	var pkgs Packages

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Split by whitespace, but respect that fields might have multiple spaces
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		snapPackageName := fields[0]
		snapPackageVersion := fields[1]

		pkg := &Package{
			Manager: SnapPackageManager,
			Name:    snapPackageName,
			Version: PackageVersion{
				Installed: snapPackageVersion,
			},
			State: PackageInstalled,
		}

		pkgs = append(pkgs, pkg)
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Join(ErrSnapPackage, err)
	}

	// Sort packages by name
	sort.SliceStable(pkgs, pkgs.ByName())

	return pkgs, nil
}

func (c *snapPackageClient) Apply(ctx context.Context, pkgs Packages) error {
	if len(pkgs) == 0 {
		// Nothing to apply
		return nil
	}

	for _, pkg := range pkgs {
		var cmd Command
		if pkg.State == PackageInstalled {
			// Install the package
			cmd = NewCommand(fmt.Sprintf(`_do() { which snap >/dev/null 2>&1; which_snap_rc=$?; if [ $which_snap_rc -ne 0 ]; then echo "which_snap_rc=${which_snap_rc}"; exit 1; fi; snap install '%s'; }; _do;`, pkg.Name))
		} else if pkg.State == PackageNotInstalled {
			// Remove the package
			cmd = NewCommand(fmt.Sprintf(`_do() { which snap >/dev/null 2>&1; which_snap_rc=$?; if [ $which_snap_rc -ne 0 ]; then echo "which_snap_rc=${which_snap_rc}"; exit 1; fi; snap remove '%s'; }; _do;`, pkg.Name))
		}

		res, err := ExecuteCommand(ctx, c.s, cmd)
		if err != nil {
			return errors.Join(ErrSnapPackageManager, err)
		}

		if strings.HasPrefix(res.StdoutString(), "which_snap_rc=") {
			return ErrSnapPackageManagerNotAvailable
		}

		if res.ExitCode != 0 {
			return errors.Join(ErrSnapPackageManager, errors.New(res.StderrString()))
		}
	}

	return nil
}
