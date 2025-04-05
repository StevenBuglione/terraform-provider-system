package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/neuspaces/terraform-provider-system/internal/client"
	"sort"
	"strings"
)

const resourcePackagesSnapName = "system_packages_snap"

const (
	resourcePackagesSnapAttrId      = "id"
	resourcePackagesSnapAttrPackage = "package"

	resourcePackagesSnapAttrPackageName = "name"

	resourcePackagesSnapAttrPackageVersions          = "versions"
	resourcePackagesSnapAttrPackageVersionsInstalled = "installed"
	resourcePackagesSnapAttrPackageVersionsAvailable = "available"
)

func resourcePackagesSnap() *schema.Resource {
	sr := &SyncResource{
		CreateContext: resourcePackagesSnapCreate,
		ReadContext:   resourcePackagesSnapRead,
		UpdateContext: resourcePackagesSnapUpdate,
		DeleteContext: resourcePackagesSnapDelete,
	}

	return &schema.Resource{
		Description: fmt.Sprintf("`%s` manages one or more snap packages on the remote system.", resourcePackagesSnapName),

		CreateContext: sr.CreateContextSync,
		ReadContext:   sr.ReadContextSync,
		UpdateContext: sr.UpdateContextSync,
		DeleteContext: sr.DeleteContextSync,

		// Importer is intentionally not configured
		// Read will not fail if the one or more packages is not installed
		// Create will implicitly import the one or more packages in the state

		SchemaVersion: 1,

		Schema: map[string]*schema.Schema{
			resourcePackagesSnapAttrId: {
				Description: "ID of the snap packages",
				Type:        schema.TypeString,
				Computed:    true,
			},
			resourcePackagesSnapAttrPackage: {
				Description: "List of packages",
				Type:        schema.TypeSet,
				Required:    true,
				MinItems:    1,
				Elem:        resourcePackagesSnapPackageSchema(),
			},
			internalDataSchemaKey: internalDataSchema(),
		},
	}
}

func resourcePackagesSnapPackageSchema() *schema.Resource {
	return &schema.Resource{
		Description: "Package description",
		Schema: map[string]*schema.Schema{
			resourcePackagesSnapAttrPackageName: {
				Description: "Name of the package",
				Type:        schema.TypeString,
				Required:    true,
			},
			resourcePackagesSnapAttrPackageVersions: {
				Description: "Computed version information of the package",
				Type:        schema.TypeList,
				Computed:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						resourcePackagesSnapAttrPackageVersionsInstalled: {
							Description: "Installed version of the package.",
							Type:        schema.TypeString,
							Computed:    true,
						},
						resourcePackagesSnapAttrPackageVersionsAvailable: {
							Description: "Available version of the package.",
							Type:        schema.TypeString,
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

type resourcePackagesSnapInternalData struct {
	// PreInstalled is map with package names as key and true as value if the package was already installed before managed by the resource
	PreInstalled map[string]bool `json:"pre_installed,omitempty"`
}

func resourcePackagesSnapIdFromPackages(pkgs client.Packages) string {
	return strings.Join(pkgs.Names(), "|")
}

func packageNamesFromResourcePackagesSnapId(id string) []string {
	return strings.Split(id, "|")
}

func expandPackagesSnapPackageSet(v interface{}) (map[string]map[string]interface{}, error) {
	pkgsSet, ok := v.(*schema.Set)
	if !ok {
		return nil, fmt.Errorf("expected *schema.Set, got unexpected type %T", v)
	}

	// Use a map with package names as key to detect duplicate package names in the set
	pkgsDataMap := make(map[string]map[string]interface{})

	for _, pkgData := range pkgsSet.List() {
		pkgDataMap, ok := pkgData.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected map[string]interface{}, got unexpected type %T", pkgData)
		}

		pkgName := pkgDataMap[resourcePackagesSnapAttrPackageName].(string)
		if pkgName == "" {
			return nil, errors.New("empty package name not allowed")
		}

		if _, duplicatePkgName := pkgsDataMap[pkgName]; duplicatePkgName {
			return nil, fmt.Errorf("duplicate package name %s", pkgName)
		}

		pkgsDataMap[pkgName] = pkgDataMap
	}

	return pkgsDataMap, nil
}

// expandPackagesSnapPackage takes a *schema.Set and returns a map of client.Package with package name as key
// expandPackagesSnapPackage raises an error on duplicate package names
func expandPackagesSnapPackage(v interface{}) (map[string]*client.Package, error) {
	pkgsDataMap, err := expandPackagesSnapPackageSet(v)
	if err != nil {
		return nil, err
	}

	pkgsMap := make(map[string]*client.Package)

	for pkgName := range pkgsDataMap {
		pkg := &client.Package{
			Manager: client.SnapPackageManager,
			Name:    pkgName,
		}

		pkgsMap[pkgName] = pkg
	}

	return pkgsMap, nil
}

func flattenPackagesSnapPackage(v client.Packages) *schema.Set {
	if v == nil {
		return nil
	}

	// Set for attribute `package`
	packageSet := schema.NewSet(schema.HashResource(resourcePackagesSnapPackageSchema()), []interface{}{})

	// Packages
	for _, clientPkg := range v {
		pkg := map[string]interface{}{}

		// Name
		pkg[resourcePackagesSnapAttrPackageName] = clientPkg.Name

		// Computed versions
		pkgVersions := make(map[string]interface{})

		if clientPkg.Version.Installed != "" {
			pkgVersions[resourcePackagesSnapAttrPackageVersionsInstalled] = clientPkg.Version.Installed
		}

		if clientPkg.Version.Available != "" {
			pkgVersions[resourcePackagesSnapAttrPackageVersionsAvailable] = clientPkg.Version.Available
		}

		pkg[resourcePackagesSnapAttrPackageVersions] = []interface{}{pkgVersions}

		packageSet.Add(pkg)
	}

	return packageSet
}

func resourcePackagesSnapGetResourceData(d *schema.ResourceData) (client.Packages, diag.Diagnostics) {
	prevPackageSet, packageSet := d.GetChange(resourcePackagesSnapAttrPackage)

	packageMap, err := expandPackagesSnapPackage(packageSet)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	prevPackageMap, err := expandPackagesSnapPackage(prevPackageSet)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	// Result package list
	pkgs := client.Packages{}

	// Packages which should be installed
	for _, pkg := range packageMap {
		installedPkg := *pkg

		// State should be installed
		installedPkg.State = client.PackageInstalled

		pkgs = append(pkgs, &installedPkg)
	}

	// Packages which should be uninstalled, i.e. are in prevPackageMap but not in packageMap
	for prevPackageName, prevPkg := range prevPackageMap {
		if _, keep := packageMap[prevPackageName]; !keep {
			uninstalledPkg := *prevPkg

			// State should be not installed
			uninstalledPkg.State = client.PackageNotInstalled

			pkgs = append(pkgs, &uninstalledPkg)
		}
	}

	// Sort packages
	sort.SliceStable(pkgs, pkgs.ByName())

	return pkgs, nil
}

func resourcePackagesSnapSetResourceData(r client.Packages, d *schema.ResourceData) diag.Diagnostics {
	_ = d.Set(resourcePackagesSnapAttrPackage, flattenPackagesSnapPackage(r))

	return nil
}

func resourcePackagesSnapNewClient(ctx context.Context, meta interface{}) (client.PackageClient, diag.Diagnostics) {
	p, diagErr := providerFromMeta(meta)
	if diagErr != nil {
		return nil, diagErr
	}

	c := client.NewSnapPackageClient(p.System)

	return c, nil
}

func resourcePackagesSnapApply(ctx context.Context, d *schema.ResourceData, meta interface{}) (client.Packages, diag.Diagnostics) {
	c, diagErr := resourcePackagesSnapNewClient(ctx, meta)
	if diagErr != nil {
		return nil, diagErr
	}

	// Get packages to determine installation state before apply
	preApplyPackages, err := c.Get(ctx)
	if err != nil {
		return nil, diag.FromErr(err)
	}
	preApplyPackagesMap := preApplyPackages.ToMap()

	r, diagErr := resourcePackagesSnapGetResourceData(d)
	if diagErr != nil {
		return nil, diagErr
	}

	err = c.Apply(ctx, r)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	// Remember installation state of the package before apply in the internal state
	var internalData resourcePackagesSnapInternalData
	_, diagErr = getInternalData(d, &internalData)
	if diagErr != nil {
		return nil, diagErr
	}

	preInstalled := internalData.PreInstalled
	if preInstalled == nil {
		preInstalled = map[string]bool{}
	}

	for _, pkg := range r {
		if pkg.State == client.PackageInstalled {
			if _, inPreInstalled := preInstalled[pkg.Name]; !inPreInstalled {
				// If package is installed and not recorded in internal data remember the pre apply state
				preApplyPkg, inPreApplyPkg := preApplyPackagesMap[pkg.Name]
				preInstalled[pkg.Name] = inPreApplyPkg && preApplyPkg.State == client.PackageInstalled
			}
		} else if pkg.State == client.PackageNotInstalled {
			// If package is uninstalled remove from internal data
			delete(preInstalled, pkg.Name)
		}
	}

	internalData.PreInstalled = preInstalled
	diagErr = setInternalData(d, &internalData)
	if diagErr != nil {
		return nil, diagErr
	}

	return r, nil
}

func resourcePackagesSnapRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, diagErr := resourcePackagesSnapNewClient(ctx, meta)
	if diagErr != nil {
		return diagErr
	}

	id := d.Id()
	packageNames := packageNamesFromResourcePackagesSnapId(id)

	r, err := c.Get(ctx)
	if err != nil {
		return diag.FromErr(err)
	}

	// Filter for relevant packages
	r = r.Filter(client.PackageNameFilter(packageNames...))

	// Filter for installed packages
	r = r.Filter(client.PackageStateFiler(client.PackageInstalled))

	diagErr = resourcePackagesSnapSetResourceData(r, d)
	if diagErr != nil {
		return diagErr
	}

	return nil
}

func resourcePackagesSnapCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	r, diagErr := resourcePackagesSnapApply(ctx, d, meta)
	if diagErr != nil {
		return diagErr
	}

	id := resourcePackagesSnapIdFromPackages(r)
	d.SetId(id)

	return resourcePackagesSnapRead(ctx, d, meta)
}

func resourcePackagesSnapUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	r, diagErr := resourcePackagesSnapApply(ctx, d, meta)
	if diagErr != nil {
		return diagErr
	}

	id := resourcePackagesSnapIdFromPackages(r)
	d.SetId(id)

	return resourcePackagesSnapRead(ctx, d, meta)
}

func resourcePackagesSnapDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, diagErr := resourcePackagesSnapNewClient(ctx, meta)
	if diagErr != nil {
		return diagErr
	}

	// In Delete func, GetChange does not provide information that the resource is deleted
	r, diagErr := resourcePackagesSnapGetResourceData(d)
	if diagErr != nil {
		return diagErr
	}

	// Restore the installation state of the packages before create
	var internalData resourcePackagesSnapInternalData
	_, diagErr = getInternalData(d, &internalData)
	if diagErr != nil {
		return diagErr
	}

	for _, pkg := range r {
		if internalData.PreInstalled != nil {
			if preApplyPkg, inPreApplyPkg := internalData.PreInstalled[pkg.Name]; inPreApplyPkg {
				if preApplyPkg {
					pkg.State = client.PackageInstalled
					continue
				}
			}
		}

		pkg.State = client.PackageNotInstalled
	}

	err := c.Apply(ctx, r)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}
