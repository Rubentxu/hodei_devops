package iam

import (
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"fmt"
	"strings"
)

const (
	// Default users
	DefaultSystemAdminUser   = "system-admin"
	DefaultOrganizationAdmin = "org-admin"
	DefaultProjectAdmin      = "project-admin"
	DefaultReadOnlyUser      = "readonly-user"
	DefaultOperatorUser      = "operator-user"

	// Default service accounts
	DefaultSystemServiceAccount     = "system-sa"
	DefaultDeploymentServiceAccount = "deploy-sa"
	DefaultMonitoringServiceAccount = "monitor-sa"
)

// CreateDefaultAdminUser creates a system administrator with full permissions
func (as *AuthorizationService) CreateDefaultAdminUser(system, organization, project string) (*model.User, error) {
	// Validate input parameters
	if err := validateParams(system, organization, project); err != nil {
		return nil, err
	}

	adminUser := &model.User{
		Name:   "System Administrator",
		Roles:  []string{RoleAdmin},
		Groups: []string{GroupAdmins},
	}

	return adminUser, nil
}

// CreateDefaultUsers creates predefined users for the project with appropriate roles and group memberships
func (as *AuthorizationService) CreateDefaultUsers(system, organization, project string, identityService *IdentityService) error {
	if err := validateParams(system, organization, project); err != nil {
		return err
	}

	if identityService == nil {
		return fmt.Errorf("identity service cannot be nil")
	}

	// System administrator user
	adminUser := &model.User{
		Name:   "System Administrator",
		Roles:  []string{RoleAdmin},
		Groups: []string{GroupAdmins},
	}
	if err := identityService.CreateUser(adminUser); err != nil {
		return fmt.Errorf("error creating admin user: %w", err)
	}

	// Organization administrator user
	orgAdminUser := &model.User{
		Name:   "Organization Administrator",
		Roles:  []string{},
		Groups: []string{GroupOrganizationAdmins},
	}
	if err := identityService.CreateUser(orgAdminUser); err != nil {
		return fmt.Errorf("error creating organization admin user: %w", err)
	}

	// Project administrator user
	projectAdminUser := &model.User{
		Name:   "Project Administrator",
		Roles:  []string{},
		Groups: []string{GroupAdmins},
	}
	if err := identityService.CreateUser(projectAdminUser); err != nil {
		return fmt.Errorf("error creating project admin user: %w", err)
	}

	// Read-only user
	readOnlyUser := &model.User{
		Name:   "Read-Only User",
		Roles:  []string{RoleProjectReader},
		Groups: []string{GroupReaders},
	}
	if err := identityService.CreateUser(readOnlyUser); err != nil {
		return fmt.Errorf("error creating read-only user: %w", err)
	}

	// Operator user
	operatorUser := &model.User{
		Name:   "Operator User",
		Roles:  []string{},
		Groups: []string{GroupOperators, GroupTaskExecutors},
	}
	if err := identityService.CreateUser(operatorUser); err != nil {
		return fmt.Errorf("error creating operator user: %w", err)
	}

	return nil
}

// CreateDefaultServiceAccounts creates predefined service accounts for the project
func (as *AuthorizationService) CreateDefaultServiceAccounts(system, organization, project string, identityService *IdentityService) error {
	if err := validateParams(system, organization, project); err != nil {
		return err
	}

	if identityService == nil {
		return fmt.Errorf("identity service cannot be nil")
	}

	// System service account
	systemSA := &model.ServiceAccount{
		Name:  "System Service Account",
		Roles: []string{RoleAdmin},
	}
	if err := identityService.CreateServiceAccount(systemSA); err != nil {
		return fmt.Errorf("error creating system service account: %w", err)
	}

	// Deployment service account
	deploySA := &model.ServiceAccount{
		Name:  "Deployment Service Account",
		Roles: []string{RoleWorkerAdmin, RoleResourcePoolAdmin, RoleTaskAdmin},
	}
	if err := identityService.CreateServiceAccount(deploySA); err != nil {
		return fmt.Errorf("error creating deployment service account: %w", err)
	}

	// Monitoring service account
	monitoringSA := &model.ServiceAccount{
		Name:  "Monitoring Service Account",
		Roles: []string{RoleWorkerViewer, RoleTaskViewer},
	}
	if err := identityService.CreateServiceAccount(monitoringSA); err != nil {
		return fmt.Errorf("error creating monitoring service account: %w", err)
	}

	return nil
}

// DefaultIdentityConfiguration creates all default users and service accounts
func (as *AuthorizationService) DefaultIdentityConfiguration(system, organization, project string, identityService *IdentityService) error {
	if err := validateParams(system, organization, project); err != nil {
		return err
	}

	if identityService == nil {
		return fmt.Errorf("identity service cannot be nil")
	}

	if err := as.CreateDefaultUsers(system, organization, project, identityService); err != nil {
		return fmt.Errorf("error configuring users: %w", err)
	}

	if err := as.CreateDefaultServiceAccounts(system, organization, project, identityService); err != nil {
		return fmt.Errorf("error configuring service accounts: %w", err)
	}

	return nil
}

// SetupDefaultSystem sets up the complete IAM system configuration with default settings
func (as *AuthorizationService) SetupDefaultSystem(system, organization, project string, identityService *IdentityService) error {
	if err := validateParams(system, organization, project); err != nil {
		return err
	}

	if identityService == nil {
		return fmt.Errorf("identity service cannot be nil")
	}

	// Configure roles and groups
	if err := as.DefaultIAMConfiguration(system, organization, project); err != nil {
		return fmt.Errorf("error configuring roles and groups: %w", err)
	}

	// Configure users and service accounts
	if err := as.DefaultIdentityConfiguration(system, organization, project, identityService); err != nil {
		return fmt.Errorf("error configuring identities: %w", err)
	}

	// Update cache
	if err := as.SyncCache(organization, project); err != nil {
		return fmt.Errorf("error syncing cache: %w", err)
	}

	return nil
}

// validateParams ensures that system, organization, and project parameters are valid
func validateParams(system, organization, project string) error {
	if strings.TrimSpace(system) == "" {
		return fmt.Errorf("system parameter cannot be empty")
	}
	if strings.TrimSpace(organization) == "" {
		return fmt.Errorf("organization parameter cannot be empty")
	}
	if strings.TrimSpace(project) == "" {
		return fmt.Errorf("project parameter cannot be empty")
	}
	return nil
}
