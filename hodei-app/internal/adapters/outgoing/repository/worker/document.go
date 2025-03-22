package workerdef_repository

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"regexp"
	"time"
)

const (
	WorkerCollection = "worker_definitions"
)

type WorkerDocument struct {
	ID       string                 `bson:"_id"`
	Metadata repository.MetadataDoc `bson:"metadata"`
	Spec     WorkerSpecDB           `bson:"spec"`
	Status   WorkerStatusDB         `bson:"status"`
}

type WorkerSpecDB struct {
	Containers    []ContainerDB     `bson:"containers"`
	Volumes       []VolumeDB        `bson:"volumes,omitempty"`
	RestartPolicy string            `bson:"restartPolicy,omitempty"`
	NodeSelector  map[string]string `bson:"nodeSelector,omitempty"`
}

type ContainerDB struct {
	Name            string                 `bson:"name"`
	Image           string                 `bson:"image"`
	Command         []string               `bson:"command,omitempty"`
	Args            []string               `bson:"args,omitempty"`
	Env             []EnvVarDB             `bson:"env,omitempty"`
	Resources       ResourceRequirementsDB `bson:"resources"`
	Ports           []PortMappingDB        `bson:"ports,omitempty"`
	VolumeMounts    []VolumeMountDB        `bson:"volumeMounts,omitempty"`
	LivenessProbe   *ProbeDB               `bson:"livenessProbe,omitempty"`
	ReadinessProbe  *ProbeDB               `bson:"readinessProbe,omitempty"`
	ImagePullPolicy string                 `bson:"imagePullPolicy,omitempty"`
	WorkingDir      string                 `bson:"workingDir,omitempty"`
}

type EnvVarDB struct {
	Name  string `bson:"name"`
	Value string `bson:"value,omitempty"`
}

type ResourceRequirementsDB struct {
	CPU    float64 `bson:"cpu"`
	Memory string  `bson:"memory"`
}

type VolumeMountDB struct {
	Name      string `bson:"name"`
	MountPath string `bson:"mountPath"`
	ReadOnly  bool   `bson:"readOnly,omitempty"`
}

type VolumeDB struct {
	Name     string            `bson:"name"`
	EmptyDir *EmptyDirVolumeDB `bson:"emptyDir,omitempty"`
	HostPath *HostPathVolumeDB `bson:"hostPath,omitempty"`
}

type EmptyDirVolumeDB struct {
	Medium    string `bson:"medium,omitempty"`
	SizeLimit string `bson:"sizeLimit,omitempty"`
}

type HostPathVolumeDB struct {
	Path string `bson:"path"`
	Type string `bson:"type,omitempty"`
}

type PortMappingDB struct {
	ContainerPort int    `bson:"containerPort"`
	Protocol      string `bson:"protocol"`
	HostPort      int    `bson:"hostPort,omitempty"`
	HostIP        string `bson:"hostIP,omitempty"`
}

type ProbeDB struct {
	Exec                *ExecActionDB      `bson:"exec,omitempty"`
	HTTPGet             *HTTPGetActionDB   `bson:"httpGet,omitempty"`
	TCPSocket           *TCPSocketActionDB `bson:"tcpSocket,omitempty"`
	InitialDelaySeconds int32              `bson:"initialDelaySeconds,omitempty"`
	TimeoutSeconds      int32              `bson:"timeoutSeconds,omitempty"`
	PeriodSeconds       int32              `bson:"periodSeconds,omitempty"`
	SuccessThreshold    int32              `bson:"successThreshold,omitempty"`
	FailureThreshold    int32              `bson:"failureThreshold,omitempty"`
}

type ExecActionDB struct {
	Command []string `bson:"command,omitempty"`
}

type HTTPGetActionDB struct {
	Path        string         `bson:"path,omitempty"`
	Port        int            `bson:"port"`
	Host        string         `bson:"host,omitempty"`
	Scheme      string         `bson:"scheme,omitempty"`
	HTTPHeaders []HTTPHeaderDB `bson:"httpHeaders,omitempty"`
}

type HTTPHeaderDB struct {
	Name  string `bson:"name"`
	Value string `bson:"value"`
}

type TCPSocketActionDB struct {
	Port int    `bson:"port"`
	Host string `bson:"host,omitempty"`
}

type WorkerStatusDB struct {
	InstanceID        string              `bson:"instanceId,omitempty"`
	State             string              `bson:"state,omitempty"`
	Message           string              `bson:"message,omitempty"`
	Reason            string              `bson:"reason,omitempty"`
	HostIP            string              `bson:"hostIP,omitempty"`
	WorkerIP          string              `bson:"workerIP,omitempty"`
	StartTime         *time.Time          `bson:"startTime,omitempty"`
	ContainerStatuses []ContainerStatusDB `bson:"containerStatuses,omitempty"`
	QOSClass          string              `bson:"qosClass,omitempty"`
}

type ContainerStatusDB struct {
	Name         string           `bson:"name"`
	Ready        bool             `bson:"ready"`
	RestartCount int32            `bson:"restartCount"`
	State        ContainerStateDB `bson:"state,omitempty"`
	Image        string           `bson:"image"`
	ImageID      string           `bson:"imageID"`
	ContainerID  string           `bson:"containerID,omitempty"`
}

type ContainerStateDB struct {
	Waiting    *ContainerStateWaitingDB    `bson:"waiting,omitempty"`
	Running    *ContainerStateRunningDB    `bson:"running,omitempty"`
	Terminated *ContainerStateTerminatedDB `bson:"terminated,omitempty"`
}

type ContainerStateWaitingDB struct {
	Reason  string `bson:"reason,omitempty"`
	Message string `bson:"message,omitempty"`
}

type ContainerStateRunningDB struct {
	StartedAt time.Time `bson:"startedAt,omitempty"`
}

type ContainerStateTerminatedDB struct {
	ExitCode    int32     `bson:"exitCode"`
	Signal      int32     `bson:"signal,omitempty"`
	Reason      string    `bson:"reason,omitempty"`
	Message     string    `bson:"message,omitempty"`
	StartedAt   time.Time `bson:"startedAt,omitempty"`
	FinishedAt  time.Time `bson:"finishedAt,omitempty"`
	ContainerID string    `bson:"containerID,omitempty"`
}

// WorkerDocumentConverter implementa la interfaz DocumentConverter para WorkerDefinition
type WorkerDocumentConverter struct {
	generator ports.IDGenerator
}

func NewWorkerDocumentConverter(generator ports.IDGenerator) generic.DocumentConverter[*model.WorkerDefinition, WorkerDocument] {
	return &WorkerDocumentConverter{
		generator: generator,
	}
}

func (c *WorkerDocumentConverter) GenerateID() model.AggregateID {
	return c.generator.NewID()
}

func (c *WorkerDocumentConverter) ToModel(doc WorkerDocument) (*model.WorkerDefinition, error) {
	// Convertir contenedores
	containers := make([]model.Container, len(doc.Spec.Containers))
	for i, c := range doc.Spec.Containers {
		// Convertir environment variables
		env := make([]model.EnvVar, len(c.Env))
		for j, e := range c.Env {
			env[j] = model.EnvVar{
				Name:  e.Name,
				Value: e.Value,
			}
		}

		// Convertir volume mounts
		volumeMounts := make([]model.VolumeMount, len(c.VolumeMounts))
		for j, vm := range c.VolumeMounts {
			volumeMounts[j] = model.VolumeMount{
				Name:      vm.Name,
				MountPath: vm.MountPath,
				ReadOnly:  vm.ReadOnly,
			}
		}

		// Convertir port mappings
		ports := make([]model.PortMapping, len(c.Ports))
		for j, p := range c.Ports {
			ports[j] = model.PortMapping{
				ContainerPort: p.ContainerPort,
				Protocol:      p.Protocol,
				HostPort:      p.HostPort,
				HostIP:        p.HostIP,
			}
		}

		// Convertir probes
		var livenessProbe, readinessProbe *model.Probe
		if c.LivenessProbe != nil {
			livenessProbe = convertProbeDBToModel(c.LivenessProbe)
		}
		if c.ReadinessProbe != nil {
			readinessProbe = convertProbeDBToModel(c.ReadinessProbe)
		}

		containers[i] = model.Container{
			Name:    c.Name,
			Image:   c.Image,
			Command: c.Command,
			Args:    c.Args,
			Env:     env,
			Resources: model.ResourceRequirements{
				CPU:    c.Resources.CPU,
				Memory: c.Resources.Memory,
			},
			Ports:           ports,
			VolumeMounts:    volumeMounts,
			LivenessProbe:   livenessProbe,
			ReadinessProbe:  readinessProbe,
			ImagePullPolicy: model.ImagePullPolicy(c.ImagePullPolicy),
			WorkingDir:      c.WorkingDir,
		}
	}

	// Convertir volumes
	volumes := make([]model.Volume, len(doc.Spec.Volumes))
	for i, v := range doc.Spec.Volumes {
		var volumeSource model.VolumeSource
		if v.EmptyDir != nil {
			volumeSource.EmptyDir = &model.EmptyDirVolumeSource{
				Medium:    v.EmptyDir.Medium,
				SizeLimit: v.EmptyDir.SizeLimit,
			}
		}
		if v.HostPath != nil {
			volumeSource.HostPath = &model.HostPathVolumeSource{
				Path: v.HostPath.Path,
				Type: model.HostPathType(v.HostPath.Type),
			}
		}
		volumes[i] = model.Volume{
			Name:         v.Name,
			VolumeSource: volumeSource,
		}
	}

	// Convertir container statuses
	containerStatuses := make([]model.ContainerStatus, len(doc.Status.ContainerStatuses))
	for i, cs := range doc.Status.ContainerStatuses {
		containerStatuses[i] = convertContainerStatusDBToModel(cs)
	}

	return &model.WorkerDefinition{
		ID: model.AggregateID(doc.ID),
		Metadata: model.Metadata{
			Name:        doc.Metadata.Name,
			Description: doc.Metadata.Description,
			Labels:      doc.Metadata.Labels,
			Annotations: doc.Metadata.Annotations,
			CreatedAt:   doc.Metadata.CreatedAt,
			UpdatedAt:   doc.Metadata.UpdatedAt,
		},
		Spec: model.WorkerSpec{
			Containers:    containers,
			Volumes:       volumes,
			RestartPolicy: model.RestartPolicy(doc.Spec.RestartPolicy),
			NodeSelector:  doc.Spec.NodeSelector,
		},
		Status: model.WorkerStatus{
			InstanceID:        doc.Status.InstanceID,
			State:             model.WorkerState(doc.Status.State),
			Message:           doc.Status.Message,
			Reason:            doc.Status.Reason,
			HostIP:            doc.Status.HostIP,
			WorkerIP:          doc.Status.WorkerIP,
			StartTime:         doc.Status.StartTime,
			ContainerStatuses: containerStatuses,
			QOSClass:          doc.Status.QOSClass,
		},
	}, nil
}

// Función auxiliar para convertir Probe
func convertProbeDBToModel(probeDB *ProbeDB) *model.Probe {
	if probeDB == nil {
		return nil
	}

	probe := &model.Probe{
		InitialDelaySeconds: probeDB.InitialDelaySeconds,
		TimeoutSeconds:      probeDB.TimeoutSeconds,
		PeriodSeconds:       probeDB.PeriodSeconds,
		SuccessThreshold:    probeDB.SuccessThreshold,
		FailureThreshold:    probeDB.FailureThreshold,
	}

	if probeDB.Exec != nil {
		probe.Exec = &model.ExecAction{
			Command: probeDB.Exec.Command,
		}
	}

	if probeDB.HTTPGet != nil {
		headers := make([]model.HTTPHeader, len(probeDB.HTTPGet.HTTPHeaders))
		for i, h := range probeDB.HTTPGet.HTTPHeaders {
			headers[i] = model.HTTPHeader{
				Name:  h.Name,
				Value: h.Value,
			}
		}
		probe.HTTPGet = &model.HTTPGetAction{
			Path:        probeDB.HTTPGet.Path,
			Port:        probeDB.HTTPGet.Port,
			Host:        probeDB.HTTPGet.Host,
			Scheme:      probeDB.HTTPGet.Scheme,
			HTTPHeaders: headers,
		}
	}

	if probeDB.TCPSocket != nil {
		probe.TCPSocket = &model.TCPSocketAction{
			Port: probeDB.TCPSocket.Port,
			Host: probeDB.TCPSocket.Host,
		}
	}

	return probe
}

// Función auxiliar para convertir ContainerStatus
func convertContainerStatusDBToModel(statusDB ContainerStatusDB) model.ContainerStatus {
	var state model.ContainerState

	if statusDB.State.Waiting != nil {
		state.Waiting = &model.ContainerStateWaiting{
			Reason:  statusDB.State.Waiting.Reason,
			Message: statusDB.State.Waiting.Message,
		}
	}

	if statusDB.State.Running != nil {
		state.Running = &model.ContainerStateRunning{
			StartedAt: statusDB.State.Running.StartedAt,
		}
	}

	if statusDB.State.Terminated != nil {
		state.Terminated = &model.ContainerStateTerminated{
			ExitCode:    statusDB.State.Terminated.ExitCode,
			Signal:      statusDB.State.Terminated.Signal,
			Reason:      statusDB.State.Terminated.Reason,
			Message:     statusDB.State.Terminated.Message,
			StartedAt:   statusDB.State.Terminated.StartedAt,
			FinishedAt:  statusDB.State.Terminated.FinishedAt,
			ContainerID: statusDB.State.Terminated.ContainerID,
		}
	}

	return model.ContainerStatus{
		Name:         statusDB.Name,
		Ready:        statusDB.Ready,
		RestartCount: statusDB.RestartCount,
		State:        state,
		Image:        statusDB.Image,
		ImageID:      statusDB.ImageID,
		ContainerID:  statusDB.ContainerID,
	}
}

func (c *WorkerDocumentConverter) ToDocument(entity *model.WorkerDefinition, ctx context.Context) WorkerDocument {
	if entity.ID == "" {
		entity.ID = c.GenerateID()
	}

	// Convertir contenedores
	containers := make([]ContainerDB, len(entity.Spec.Containers))
	for i, c := range entity.Spec.Containers {
		// Convertir variables de entorno
		env := make([]EnvVarDB, len(c.Env))
		for j, e := range c.Env {
			env[j] = EnvVarDB{
				Name:  e.Name,
				Value: e.Value,
			}
		}

		// Convertir montajes de volúmenes
		volumeMounts := make([]VolumeMountDB, len(c.VolumeMounts))
		for j, vm := range c.VolumeMounts {
			volumeMounts[j] = VolumeMountDB{
				Name:      vm.Name,
				MountPath: vm.MountPath,
				ReadOnly:  vm.ReadOnly,
			}
		}

		// Convertir mapeos de puertos
		ports := make([]PortMappingDB, len(c.Ports))
		for j, p := range c.Ports {
			ports[j] = PortMappingDB{
				ContainerPort: p.ContainerPort,
				Protocol:      p.Protocol,
				HostPort:      p.HostPort,
				HostIP:        p.HostIP,
			}
		}

		// Convertir probes
		var livenessProbe, readinessProbe *ProbeDB
		if c.LivenessProbe != nil {
			livenessProbe = convertProbeToDB(c.LivenessProbe)
		}
		if c.ReadinessProbe != nil {
			readinessProbe = convertProbeToDB(c.ReadinessProbe)
		}

		containers[i] = ContainerDB{
			Name:    c.Name,
			Image:   c.Image,
			Command: c.Command,
			Args:    c.Args,
			Env:     env,
			Resources: ResourceRequirementsDB{
				CPU:    c.Resources.CPU,
				Memory: c.Resources.Memory,
			},
			Ports:           ports,
			VolumeMounts:    volumeMounts,
			LivenessProbe:   livenessProbe,
			ReadinessProbe:  readinessProbe,
			ImagePullPolicy: string(c.ImagePullPolicy),
			WorkingDir:      c.WorkingDir,
		}
	}

	// Convertir volúmenes
	volumes := make([]VolumeDB, len(entity.Spec.Volumes))
	for i, v := range entity.Spec.Volumes {
		volume := VolumeDB{
			Name: v.Name,
		}
		if v.VolumeSource.EmptyDir != nil {
			volume.EmptyDir = &EmptyDirVolumeDB{
				Medium:    v.VolumeSource.EmptyDir.Medium,
				SizeLimit: v.VolumeSource.EmptyDir.SizeLimit,
			}
		}
		if v.VolumeSource.HostPath != nil {
			volume.HostPath = &HostPathVolumeDB{
				Path: v.VolumeSource.HostPath.Path,
				Type: string(v.VolumeSource.HostPath.Type),
			}
		}
		volumes[i] = volume
	}

	// Convertir estados de contenedores
	containerStatuses := make([]ContainerStatusDB, len(entity.Status.ContainerStatuses))
	for i, cs := range entity.Status.ContainerStatuses {
		containerStatuses[i] = convertContainerStatusToDB(cs)
	}

	return WorkerDocument{
		ID: entity.ID.String(),
		Metadata: repository.MetadataDoc{
			Name:        entity.Metadata.Name,
			Description: entity.Metadata.Description,
			Labels:      entity.Metadata.Labels,
			Annotations: entity.Metadata.Annotations,
			CreatedAt:   entity.Metadata.CreatedAt,
			UpdatedAt:   entity.Metadata.UpdatedAt,
		},
		Spec: WorkerSpecDB{
			Containers:    containers,
			Volumes:       volumes,
			RestartPolicy: string(entity.Spec.RestartPolicy),
			NodeSelector:  entity.Spec.NodeSelector,
		},
		Status: WorkerStatusDB{
			InstanceID:        entity.Status.InstanceID,
			State:             string(entity.Status.State),
			Message:           entity.Status.Message,
			Reason:            entity.Status.Reason,
			HostIP:            entity.Status.HostIP,
			WorkerIP:          entity.Status.WorkerIP,
			StartTime:         entity.Status.StartTime,
			ContainerStatuses: containerStatuses,
			QOSClass:          entity.Status.QOSClass,
		},
	}
}

// Función auxiliar para convertir Probe a ProbeDB
func convertProbeToDB(probe *model.Probe) *ProbeDB {
	if probe == nil {
		return nil
	}

	probeDB := &ProbeDB{
		InitialDelaySeconds: probe.InitialDelaySeconds,
		TimeoutSeconds:      probe.TimeoutSeconds,
		PeriodSeconds:       probe.PeriodSeconds,
		SuccessThreshold:    probe.SuccessThreshold,
		FailureThreshold:    probe.FailureThreshold,
	}

	if probe.Exec != nil {
		probeDB.Exec = &ExecActionDB{
			Command: probe.Exec.Command,
		}
	}

	if probe.HTTPGet != nil {
		headers := make([]HTTPHeaderDB, len(probe.HTTPGet.HTTPHeaders))
		for i, h := range probe.HTTPGet.HTTPHeaders {
			headers[i] = HTTPHeaderDB{
				Name:  h.Name,
				Value: h.Value,
			}
		}
		probeDB.HTTPGet = &HTTPGetActionDB{
			Path:        probe.HTTPGet.Path,
			Port:        probe.HTTPGet.Port,
			Host:        probe.HTTPGet.Host,
			Scheme:      probe.HTTPGet.Scheme,
			HTTPHeaders: headers,
		}
	}

	if probe.TCPSocket != nil {
		probeDB.TCPSocket = &TCPSocketActionDB{
			Port: probe.TCPSocket.Port,
			Host: probe.TCPSocket.Host,
		}
	}

	return probeDB
}

// Función auxiliar para convertir ContainerStatus a ContainerStatusDB
func convertContainerStatusToDB(status model.ContainerStatus) ContainerStatusDB {
	var state ContainerStateDB

	if status.State.Waiting != nil {
		state.Waiting = &ContainerStateWaitingDB{
			Reason:  status.State.Waiting.Reason,
			Message: status.State.Waiting.Message,
		}
	}

	if status.State.Running != nil {
		state.Running = &ContainerStateRunningDB{
			StartedAt: status.State.Running.StartedAt,
		}
	}

	if status.State.Terminated != nil {
		state.Terminated = &ContainerStateTerminatedDB{
			ExitCode:    status.State.Terminated.ExitCode,
			Signal:      status.State.Terminated.Signal,
			Reason:      status.State.Terminated.Reason,
			Message:     status.State.Terminated.Message,
			StartedAt:   status.State.Terminated.StartedAt,
			FinishedAt:  status.State.Terminated.FinishedAt,
			ContainerID: status.State.Terminated.ContainerID,
		}
	}

	return ContainerStatusDB{
		Name:         status.Name,
		Ready:        status.Ready,
		RestartCount: status.RestartCount,
		State:        state,
		Image:        status.Image,
		ImageID:      status.ImageID,
		ContainerID:  status.ContainerID,
	}
}

// BuildFilter construye un filtro BSON a partir de los criterios de búsqueda
func (c *WorkerDocumentConverter) BuildFilter(filters map[string]interface{}) bson.M {
	if filters == nil || len(filters) == 0 {
		return bson.M{}
	}

	filter := bson.M{}

	for key, value := range filters {
		switch key {
		case "name":
			filter["metadata.name"] = value
		case "type":
			filter["spec.type"] = value
		case "image":
			filter["spec.image"] = value
		case "status":
			filter["status.status"] = value
		case "owner":
			filter["owner"] = value
		case "tenantId":
			filter["tenant_id"] = value
		case "labels":
			if labels, ok := value.([]string); ok && len(labels) > 0 {
				filter["metadata.labels"] = bson.M{"$all": labels}
			}
		case "nameContains":
			if strValue, ok := value.(string); ok {
				filter["metadata.name"] = bson.M{"$regex": primitive.Regex{
					Pattern: regexp.QuoteMeta(strValue),
					Options: "i",
				}}
			}
		case "descriptionContains":
			if strValue, ok := value.(string); ok {
				filter["metadata.description"] = bson.M{"$regex": primitive.Regex{
					Pattern: regexp.QuoteMeta(strValue),
					Options: "i",
				}}
			}
		case "templateId":
			filter["spec.template_id"] = value
		}
	}

	return filter
}

// MapSortField mapea el nombre de campo para ordenamiento
func (c *WorkerDocumentConverter) MapSortField(field string) string {
	switch field {
	case "name":
		return "metadata.name"
	case "type":
		return "spec.type"
	case "status":
		return "status.status"
	case "createdAt":
		return "created_at"
	case "updatedAt":
		return "updated_at"
	default:
		return "_id"
	}
}
