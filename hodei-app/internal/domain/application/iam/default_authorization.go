package iam

import (
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"fmt"
)

// Resource Types define the primary resources in the system
const (
	// Resource Types
	ResourceTypeWorker        = "worker"        // Worker resource type
	ResourceTypeResourcePool  = "resourcepool"  // ResourcePool resource type
	ResourceTypeTask          = "task"          // Task resource type
	ResourceTypeTaskExecution = "taskexecution" // TaskExecution resource type

	// Worker specific roles
	RoleWorkerAdmin    = "WorkerAdmin"    // Can create, modify and delete workers
	RoleWorkerCreator  = "WorkerCreator"  // Can create and modify workers but not delete them
	RoleWorkerOperator = "WorkerOperator" // Can operate workers but not modify their definition
	RoleWorkerViewer   = "WorkerViewer"   // Can only view workers

	// ResourcePool specific roles
	RoleResourcePoolAdmin = "ResourcePoolAdmin" // Can manage resource pools completely
	RoleResourcePoolUser  = "ResourcePoolUser"  // Can use resource pools but not manage them

	RoleAdmin           = "Admin"           // Full access to all project resources
	RoleProjectReader   = "ProjectReader"   // Read-only access to project resources
	RoleProjectEditor   = "ProjectEditor"   // Read, write and delete access
	RoleProjectOperator = "ProjectOperator" // Can operate but not modify (read + limited write)

	// Task specific roles
	RoleTaskAdmin    = "TaskAdmin"    // Can manage all aspects of tasks
	RoleTaskCreator  = "TaskCreator"  // Can create and modify tasks
	RoleTaskExecutor = "TaskExecutor" // Can execute tasks but not modify their definition
	RoleTaskViewer   = "TaskViewer"   // Can only view tasks and their executions

	// Worker groups
	GroupWorkerAdmins   = "WorkerAdmins"   // Administrators of Workers
	GroupWorkerCreators = "WorkerCreators" // Can create and manage workers
	GroupWorkerUsers    = "WorkerUsers"    // Can use but not modify workers

	// ResourcePool groups
	GroupPoolAdmins = "ResourcePoolAdmins" // Administrators of ResourcePools
	GroupPoolUsers  = "ResourcePoolUsers"  // Users of ResourcePools

	// Task groups
	GroupTaskAdmins    = "TaskAdmins"    // Administrators of Tasks
	GroupTaskCreators  = "TaskCreators"  // Can create tasks
	GroupTaskExecutors = "TaskExecutors" // Can execute tasks

	GroupAdmins    = "Admins"    // Project administrators
	GroupEditors   = "Editors"   // Project editors
	GroupReaders   = "Readers"   // Project readers
	GroupOperators = "Operators" // Project operators
	GroupAuditors  = "Auditors"  // Project auditors

	// Organization groups
	GroupOwners             = "Owners"             // Organization owners
	GroupOrganizationAdmins = "OrganizationAdmins" // Organization administrators
	GroupBilling            = "Billing"            // Billing team
)

// createWorkerRoles creates roles specific to Worker resources
func (as *AuthorizationService) createWorkerRoles(system, organization, project string) error {
	// WorkerAdmin role - Complete control over workers
	workerAdminRole := model.Role{
		Name:         RoleWorkerAdmin,
		System:       system,
		Organization: organization,
		Project:      project,
		Permissions: []model.Permission{
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     ResourceTypeWorker,
				},
				AllowedActions: []model.Action{model.ActionRead, model.ActionWrite, model.ActionDelete, model.ActionAdmin},
			},
		},
	}
	if err := as.CreateRole(workerAdminRole); err != nil {
		return fmt.Errorf("error creating WorkerAdmin role: %w", err)
	}

	// WorkerCreator role - Can create and modify but not delete
	workerCreatorRole := model.Role{
		Name:         RoleWorkerCreator,
		System:       system,
		Organization: organization,
		Project:      project,
		Permissions: []model.Permission{
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     ResourceTypeWorker,
				},
				AllowedActions: []model.Action{model.ActionRead, model.ActionWrite},
			},
		},
	}
	if err := as.CreateRole(workerCreatorRole); err != nil {
		return fmt.Errorf("error creating WorkerCreator role: %w", err)
	}

	// WorkerOperator role - Can operate workers but not modify them
	workerOperatorRole := model.Role{
		Name:         RoleWorkerOperator,
		System:       system,
		Organization: organization,
		Project:      project,
		Permissions: []model.Permission{
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     ResourceTypeWorker,
				},
				AllowedActions: []model.Action{model.ActionRead},
			},
		},
	}
	if err := as.CreateRole(workerOperatorRole); err != nil {
		return fmt.Errorf("error creating WorkerOperator role: %w", err)
	}

	// WorkerViewer role - Read-only access to workers
	workerViewerRole := model.Role{
		Name:         RoleWorkerViewer,
		System:       system,
		Organization: organization,
		Project:      project,
		Permissions: []model.Permission{
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     ResourceTypeWorker,
				},
				AllowedActions: []model.Action{model.ActionRead},
			},
		},
	}
	if err := as.CreateRole(workerViewerRole); err != nil {
		return fmt.Errorf("error creating WorkerViewer role: %w", err)
	}

	return nil
}

// createResourcePoolRoles creates roles specific to ResourcePool resources
func (as *AuthorizationService) createResourcePoolRoles(system, organization, project string) error {
	// ResourcePoolAdmin role - Complete control over resource pools
	resourcePoolAdminRole := model.Role{
		Name:         RoleResourcePoolAdmin,
		System:       system,
		Organization: organization,
		Project:      project,
		Permissions: []model.Permission{
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     ResourceTypeResourcePool,
				},
				AllowedActions: []model.Action{model.ActionRead, model.ActionWrite, model.ActionDelete, model.ActionAdmin},
			},
		},
	}
	if err := as.CreateRole(resourcePoolAdminRole); err != nil {
		return fmt.Errorf("error creating ResourcePoolAdmin role: %w", err)
	}

	// ResourcePoolUser role - Can use resource pools but not manage them
	resourcePoolUserRole := model.Role{
		Name:         RoleResourcePoolUser,
		System:       system,
		Organization: organization,
		Project:      project,
		Permissions: []model.Permission{
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     ResourceTypeResourcePool,
				},
				AllowedActions: []model.Action{model.ActionRead},
			},
		},
	}
	if err := as.CreateRole(resourcePoolUserRole); err != nil {
		return fmt.Errorf("error creating ResourcePoolUser role: %w", err)
	}

	return nil
}

// createTaskRoles creates roles specific to Task resources
func (as *AuthorizationService) createTaskRoles(system, organization, project string) error {
	// TaskAdmin role - Complete control over tasks and their executions
	taskAdminRole := model.Role{
		Name:         RoleTaskAdmin,
		System:       system,
		Organization: organization,
		Project:      project,
		Permissions: []model.Permission{
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     ResourceTypeTask,
				},
				AllowedActions: []model.Action{model.ActionRead, model.ActionWrite, model.ActionDelete, model.ActionAdmin},
			},
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     ResourceTypeTaskExecution,
				},
				AllowedActions: []model.Action{model.ActionRead, model.ActionWrite, model.ActionDelete, model.ActionAdmin},
			},
		},
	}
	if err := as.CreateRole(taskAdminRole); err != nil {
		return fmt.Errorf("error creating TaskAdmin role: %w", err)
	}

	// TaskCreator role - Can create and modify tasks
	taskCreatorRole := model.Role{
		Name:         RoleTaskCreator,
		System:       system,
		Organization: organization,
		Project:      project,
		Permissions: []model.Permission{
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     ResourceTypeTask,
				},
				AllowedActions: []model.Action{model.ActionRead, model.ActionWrite, model.ActionDelete},
			},
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     ResourceTypeTaskExecution,
				},
				AllowedActions: []model.Action{model.ActionRead},
			},
		},
	}
	if err := as.CreateRole(taskCreatorRole); err != nil {
		return fmt.Errorf("error creating TaskCreator role: %w", err)
	}

	// TaskExecutor role - Can execute tasks but not modify their definition
	taskExecutorRole := model.Role{
		Name:         RoleTaskExecutor,
		System:       system,
		Organization: organization,
		Project:      project,
		Permissions: []model.Permission{
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     ResourceTypeTask,
				},
				AllowedActions: []model.Action{model.ActionRead},
			},
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     ResourceTypeTaskExecution,
				},
				AllowedActions: []model.Action{model.ActionRead, model.ActionWrite},
			},
		},
	}
	if err := as.CreateRole(taskExecutorRole); err != nil {
		return fmt.Errorf("error creating TaskExecutor role: %w", err)
	}

	// TaskViewer role - Can only view tasks and executions
	taskViewerRole := model.Role{
		Name:         RoleTaskViewer,
		System:       system,
		Organization: organization,
		Project:      project,
		Permissions: []model.Permission{
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     ResourceTypeTask,
				},
				AllowedActions: []model.Action{model.ActionRead},
			},
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     ResourceTypeTaskExecution,
				},
				AllowedActions: []model.Action{model.ActionRead},
			},
		},
	}
	if err := as.CreateRole(taskViewerRole); err != nil {
		return fmt.Errorf("error creating TaskViewer role: %w", err)
	}

	return nil
}

func (as *AuthorizationService) createResourceGroups(system, organization, project string) error {
	// Worker groups
	workerAdminsGroup := model.Group{
		System:       system,
		Name:         GroupWorkerAdmins,
		Organization: organization,
		Project:      project,
		Roles:        []string{RoleWorkerAdmin},
		Members:      []string{},
	}
	if err := as.CreateGroup(workerAdminsGroup); err != nil {
		return fmt.Errorf("error creating WorkerAdmins group: %w", err)
	}

	workerCreatorsGroup := model.Group{
		System:       system,
		Name:         GroupWorkerCreators,
		Organization: organization,
		Project:      project,
		Roles:        []string{RoleWorkerCreator},
		Members:      []string{},
	}
	if err := as.CreateGroup(workerCreatorsGroup); err != nil {
		return fmt.Errorf("error creating WorkerCreators group: %w", err)
	}

	workerUsersGroup := model.Group{
		System:       system,
		Name:         GroupWorkerUsers,
		Organization: organization,
		Project:      project,
		Roles:        []string{RoleWorkerOperator},
		Members:      []string{},
	}
	if err := as.CreateGroup(workerUsersGroup); err != nil {
		return fmt.Errorf("error creating WorkerUsers group: %w", err)
	}

	// ResourcePool groups
	poolAdminsGroup := model.Group{
		Name:         GroupPoolAdmins,
		Organization: organization,
		Project:      project,
		Roles:        []string{RoleResourcePoolAdmin},
		Members:      []string{},
	}
	if err := as.CreateGroup(poolAdminsGroup); err != nil {
		return fmt.Errorf("error creating ResourcePoolAdmins group: %w", err)
	}

	poolUsersGroup := model.Group{
		System:       system,
		Name:         GroupPoolUsers,
		Organization: organization,
		Project:      project,
		Roles:        []string{RoleResourcePoolUser},
		Members:      []string{},
	}
	if err := as.CreateGroup(poolUsersGroup); err != nil {
		return fmt.Errorf("error creating ResourcePoolUsers group: %w", err)
	}

	// Task groups
	taskAdminsGroup := model.Group{
		System:       system,
		Name:         GroupTaskAdmins,
		Organization: organization,
		Project:      project,
		Roles:        []string{RoleTaskAdmin},
		Members:      []string{},
	}
	if err := as.CreateGroup(taskAdminsGroup); err != nil {
		return fmt.Errorf("error creating TaskAdmins group: %w", err)
	}

	taskCreatorsGroup := model.Group{
		System:       system,
		Name:         GroupTaskCreators,
		Organization: organization,
		Project:      project,
		Roles:        []string{RoleTaskCreator},
		Members:      []string{},
	}
	if err := as.CreateGroup(taskCreatorsGroup); err != nil {
		return fmt.Errorf("error creating TaskCreators group: %w", err)
	}

	taskExecutorsGroup := model.Group{
		System:       system,
		Name:         GroupTaskExecutors,
		Organization: organization,
		Project:      project,
		Roles:        []string{RoleTaskExecutor},
		Members:      []string{},
	}
	if err := as.CreateGroup(taskExecutorsGroup); err != nil {
		return fmt.Errorf("error creating TaskExecutors group: %w", err)
	}

	return nil
}

// DefaultResourceConfiguration creates the default resource-specific roles and groups for a project
func (as *AuthorizationService) DefaultResourceConfiguration(system, organization, project string) error {
	// Create resource-specific roles
	if err := as.createWorkerRoles(system, organization, project); err != nil {
		return fmt.Errorf("error creating worker roles: %w", err)
	}

	if err := as.createResourcePoolRoles(system, organization, project); err != nil {
		return fmt.Errorf("error creating resource pool roles: %w", err)
	}

	if err := as.createTaskRoles(system, organization, project); err != nil {
		return fmt.Errorf("error creating task roles: %w", err)
	}

	// Create resource-specific groups
	if err := as.createResourceGroups(system, organization, project); err != nil {
		return fmt.Errorf("error creating resource groups: %w", err)
	}

	return nil
}

func (as *AuthorizationService) DefaultProjectConfiguration(system, organization, project string) error {

	// Rol Admin: Control total (ActionAdmin) sobre todos los recursos del proyecto.
	adminRole := model.Role{
		Name:         RoleAdmin,
		System:       system,
		Organization: organization,
		Project:      project,
		Permissions: []model.Permission{
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     "*",
				},
				AllowedActions: []model.Action{model.ActionAdmin},
			},
		},
	}
	if err := as.CreateRole(adminRole); err != nil {
		return fmt.Errorf("Error in CreateRole for AdminRole: %v", err)
	}

	// Rol ProjectReader: Solo lectura sobre todos los recursos del proyecto.
	readerRole := model.Role{
		Name:         RoleProjectReader,
		System:       system,
		Organization: organization,
		Project:      project,
		Permissions: []model.Permission{
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     "*",
				},
				AllowedActions: []model.Action{model.ActionRead},
			},
		},
	}
	if err := as.CreateRole(readerRole); err != nil {
		return fmt.Errorf("Error in CreateRole for ReaderRole: %v", err)
	}

	// Rol ProjectEditor: Permite lectura, escritura y eliminación.
	editorRole := model.Role{
		Name:         RoleProjectEditor,
		System:       system,
		Organization: organization,
		Project:      project,
		Permissions: []model.Permission{
			{
				ResourcePattern: model.ResourceURN{
					System:       system,
					Organization: organization,
					Project:      project,
					Resource:     "*",
				},
				AllowedActions: []model.Action{model.ActionRead, model.ActionWrite, model.ActionDelete},
			},
		},
	}
	if err := as.CreateRole(editorRole); err != nil {
		return fmt.Errorf("Error in CreateRole for EditorRole: %v", err)
	}

	// Grupo "Admins": asigna el rol Admin.
	adminsGroup := model.Group{
		System:       system,
		Name:         GroupAdmins,
		Organization: organization,
		Project:      project,
		Roles:        []string{RoleAdmin},
		Members:      []string{},
	}
	if err := as.CreateGroup(adminsGroup); err != nil {
		return fmt.Errorf("Error in CreateGroup for AdminsGroup: %v", err)
	}

	// Grupo "Editors": asigna el rol ProjectEditor.
	editorsGroup := model.Group{
		Name:         GroupEditors,
		System:       system,
		Organization: organization,
		Project:      project,
		Roles:        []string{RoleProjectEditor},
		Members:      []string{},
	}
	if err := as.CreateGroup(editorsGroup); err != nil {
		return fmt.Errorf("Error in CreateGroup for EditorsGroup: %v", err)
	}
	return nil
}

// DefaultIAMConfiguration actualiza la función DefaultProjectConfiguration para incluir
// la configuración de roles y grupos específicos por recurso
func (as *AuthorizationService) DefaultIAMConfiguration(system, organization, project string) error {
	// Crear los roles y grupos estándar
	if err := as.DefaultProjectConfiguration(system, organization, project); err != nil {
		return err
	}

	// Añadir los roles y grupos específicos por recurso
	if err := as.DefaultResourceConfiguration(system, organization, project); err != nil {
		return err
	}

	return nil
}
