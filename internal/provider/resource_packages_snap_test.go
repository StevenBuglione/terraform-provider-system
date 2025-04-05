package provider_test

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/neuspaces/terraform-provider-system/internal/acctest"
	"github.com/neuspaces/terraform-provider-system/internal/acctest/tfbuild"
	"github.com/neuspaces/terraform-provider-system/internal/lib/osrelease"
	"github.com/neuspaces/terraform-provider-system/internal/provider"
	"regexp"
	"testing"
)

// Test to install a single snap package which is not installed
//
// Preconditions:
// - Package `hello` is not installed
//
// Expected:
// - Package `hello` is installed after create
// - Package `hello` is not installed after destroy
func TestAccPackagesSnap_create_single(t *testing.T) {
	acctest.Current().Targets.Foreach(t, func(t *testing.T, target acctest.Target) {
		t.Parallel()

		// Skip if snap is not available
		acctest.SkipWhenOSNotEquals(t, target, osrelease.DebianId)

		resource.Test(t, resource.TestCase{
			ProviderFactories: acctest.ProviderFactories(),
			Steps: []resource.TestStep{
				{
					Config: tfbuild.FileString(tfbuild.File(
						acctest.ProviderConfigBlock(target.Configs.Default()),
						testAccPackageSnapBlock("test",
							tfbuild.InnerBlock("package",
								tfbuild.AttributeString("name", "hello"),
							),
						),
					)),
					Check: resource.ComposeTestCheckFunc(
						provider.TestLogResourceAttr(t, "system_packages_snap.test"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "id"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.#", "1"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.name", "hello"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.versions.#", "1"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "package.0.versions.0.installed"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.versions.0.available", ""),
						provider.TestCheckResourceAttrBase64("system_packages_snap.test", "internal", `{"pre_installed":{"hello":false}}`),
					),
				},
			},
		})
	})
}

// Test to install a single snap package which is already installed
//
// Preconditions:
// - Package `core` is installed
//
// Expected:
// - Package `core` is installed after create
// - Package `core` is installed after destroy
func TestAccPackagesSnap_create_single_idempotent(t *testing.T) {
	acctest.Current().Targets.Foreach(t, func(t *testing.T, target acctest.Target) {
		t.Parallel()

		// Skip if snap is not available
		acctest.SkipWhenOSNotEquals(t, target, osrelease.DebianId)

		resource.Test(t, resource.TestCase{
			ProviderFactories: acctest.ProviderFactories(),
			Steps: []resource.TestStep{
				{
					Config: tfbuild.FileString(tfbuild.File(
						acctest.ProviderConfigBlock(target.Configs.Default()),
						testAccPackageSnapBlock("test",
							tfbuild.InnerBlock("package",
								tfbuild.AttributeString("name", "core"),
							),
						),
					)),
					Check: resource.ComposeTestCheckFunc(
						provider.TestLogResourceAttr(t, "system_packages_snap.test"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "id"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.#", "1"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.name", "core"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.versions.#", "1"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "package.0.versions.0.installed"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.versions.0.available", ""),
						provider.TestCheckResourceAttrBase64("system_packages_snap.test", "internal", `{"pre_installed":{"core":true}}`),
					),
				},
			},
		})
	})
}

// Test to install multiple snap packages with mixed preconditions
//
// Preconditions:
// - Package `hello` is not installed
// - Package `core` is installed
//
// Expected:
// - Package `hello` is installed after create
// - Package `hello` is not installed after destroy
// - Package `core` is installed after create
// - Package `core` is installed after destroy
func TestAccPackagesSnap_multiple(t *testing.T) {
	acctest.Current().Targets.Foreach(t, func(t *testing.T, target acctest.Target) {
		t.Parallel()

		// Skip if snap is not available
		acctest.SkipWhenOSNotEquals(t, target, osrelease.DebianId)

		resource.Test(t, resource.TestCase{
			ProviderFactories: acctest.ProviderFactories(),
			Steps: []resource.TestStep{
				{
					Config: tfbuild.FileString(tfbuild.File(
						acctest.ProviderConfigBlock(target.Configs.Default()),
						testAccPackageSnapBlock("test",
							tfbuild.InnerBlock("package",
								tfbuild.AttributeString("name", "hello"),
							),
							tfbuild.InnerBlock("package",
								tfbuild.AttributeString("name", "core"),
							),
						),
					)),
					Check: resource.ComposeTestCheckFunc(
						provider.TestLogResourceAttr(t, "system_packages_snap.test"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "id"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.#", "2"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.name", "core"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.versions.#", "1"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "package.0.versions.0.installed"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.versions.0.available", ""),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.1.name", "hello"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.1.versions.#", "1"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "package.1.versions.0.installed"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.1.versions.0.available", ""),
						provider.TestCheckResourceAttrBase64("system_packages_snap.test", "internal", `{"pre_installed":{"core":true,"hello":false}}`),
					),
				},
			},
		})
	})
}

// Test to install a single snap package and add a second snap package
//
// Preconditions:
// - Package `hello` is not installed
// - Package `hello-world` is not installed
//
// Expected:
// - Package `hello` is installed after create
// - Package `hello-world` is installed after update
// - Package `hello` is not installed after destroy
// - Package `hello-world` is not installed after destroy
func TestAccPackagesSnap_update_add_package(t *testing.T) {
	acctest.Current().Targets.Foreach(t, func(t *testing.T, target acctest.Target) {
		t.Parallel()

		// Skip if snap is not available
		acctest.SkipWhenOSNotEquals(t, target, osrelease.DebianId)

		resource.Test(t, resource.TestCase{
			ProviderFactories: acctest.ProviderFactories(),
			Steps: []resource.TestStep{
				{
					Config: provider.TestLogString(t, tfbuild.FileString(tfbuild.File(
						acctest.ProviderConfigBlock(target.Configs.Default()),
						testAccPackageSnapBlock("test",
							tfbuild.InnerBlock("package",
								tfbuild.AttributeString("name", "hello"),
							),
						),
					))),
					Check: resource.ComposeTestCheckFunc(
						provider.TestLogResourceAttr(t, "system_packages_snap.test"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "id"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.#", "1"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.name", "hello"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "package.0.versions.0.installed"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.versions.0.available", ""),
						provider.TestCheckResourceAttrBase64("system_packages_snap.test", "internal", `{"pre_installed":{"hello":false}}`),
					),
				},
				{
					Config: provider.TestLogString(t, tfbuild.FileString(tfbuild.File(
						acctest.ProviderConfigBlock(target.Configs.Default()),
						testAccPackageSnapBlock("test",
							tfbuild.InnerBlock("package",
								tfbuild.AttributeString("name", "hello"),
							),
							tfbuild.InnerBlock("package",
								tfbuild.AttributeString("name", "hello-world"),
							),
						),
					))),
					Check: resource.ComposeTestCheckFunc(
						provider.TestLogResourceAttr(t, "system_packages_snap.test"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "id"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.#", "2"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.name", "hello"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "package.0.versions.0.installed"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.versions.0.available", ""),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.1.name", "hello-world"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "package.1.versions.0.installed"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.1.versions.0.available", ""),
						provider.TestCheckResourceAttrBase64("system_packages_snap.test", "internal", `{"pre_installed":{"hello":false,"hello-world":false}}`),
					),
				},
			},
		})
	})
}

// Test to install multiple snap packages and remove one
//
// Preconditions:
// - Package `hello` is not installed
// - Package `hello-world` is not installed
//
// Expected:
// - Package `hello` is installed after create
// - Package `hello-world` is installed after create
// - Package `hello` is installed after update
// - Package `hello-world` is not installed after update
// - Package `hello` is not installed after destroy
func TestAccPackagesSnap_update_remove_package(t *testing.T) {
	acctest.Current().Targets.Foreach(t, func(t *testing.T, target acctest.Target) {
		t.Parallel()

		// Skip if snap is not available
		acctest.SkipWhenOSNotEquals(t, target, osrelease.DebianId)

		resource.Test(t, resource.TestCase{
			ProviderFactories: acctest.ProviderFactories(),
			Steps: []resource.TestStep{
				{
					Config: provider.TestLogString(t, tfbuild.FileString(tfbuild.File(
						acctest.ProviderConfigBlock(target.Configs.Default()),
						testAccPackageSnapBlock("test",
							tfbuild.InnerBlock("package",
								tfbuild.AttributeString("name", "hello"),
							),
							tfbuild.InnerBlock("package",
								tfbuild.AttributeString("name", "hello-world"),
							),
						),
					))),
					Check: resource.ComposeTestCheckFunc(
						provider.TestLogResourceAttr(t, "system_packages_snap.test"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "id"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.#", "2"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.name", "hello"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "package.0.versions.0.installed"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.versions.0.available", ""),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.1.name", "hello-world"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "package.1.versions.0.installed"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.1.versions.0.available", ""),
						provider.TestCheckResourceAttrBase64("system_packages_snap.test", "internal", `{"pre_installed":{"hello":false,"hello-world":false}}`),
					),
				},
				{
					Config: provider.TestLogString(t, tfbuild.FileString(tfbuild.File(
						acctest.ProviderConfigBlock(target.Configs.Default()),
						testAccPackageSnapBlock("test",
							tfbuild.InnerBlock("package",
								tfbuild.AttributeString("name", "hello"),
							),
						),
					))),
					Check: resource.ComposeTestCheckFunc(
						provider.TestLogResourceAttr(t, "system_packages_snap.test"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "id"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.#", "1"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.name", "hello"),
						resource.TestCheckResourceAttrSet("system_packages_snap.test", "package.0.versions.0.installed"),
						resource.TestCheckResourceAttr("system_packages_snap.test", "package.0.versions.0.available", ""),
						provider.TestCheckResourceAttrBase64("system_packages_snap.test", "internal", `{"pre_installed":{"hello":false}}`),
					),
				},
			},
		})
	})
}

// Test behavior when snap is unavailable
//
// Expected:
// - Test should fail with error about snap not being available
func TestAccPackagesSnap_unavailable(t *testing.T) {
	acctest.Current().Targets.Foreach(t, func(t *testing.T, target acctest.Target) {
		t.Parallel()

		// Only run this test if snap is not available
		acctest.SkipWhenOSNotEquals(t, target, osrelease.DebianId)

		resource.Test(t, resource.TestCase{
			ProviderFactories: acctest.ProviderFactories(),
			Steps: []resource.TestStep{
				{
					Config: tfbuild.FileString(tfbuild.File(
						acctest.ProviderConfigBlock(target.Configs.Default()),
						testAccPackageSnapBlock("test",
							tfbuild.InnerBlock("package",
								tfbuild.AttributeString("name", "hello"),
							),
						),
					)),
					ExpectError: regexp.MustCompile(`snap not available`),
				},
			},
		})
	})
}

func testAccPackageSnapBlock(name string, attrs ...tfbuild.BlockElement) tfbuild.FileElement {
	return tfbuild.Resource("system_packages_snap", name, attrs...)
}
