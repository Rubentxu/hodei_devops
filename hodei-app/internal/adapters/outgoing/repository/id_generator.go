package generator_id

import (
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var _ ports.IDGenerator = (*ObjectIDGenerator)(nil)

type ObjectIDGenerator struct{}

// NewID genera un ObjectId y lo retorna en su representación hexadecimal.
func (g *ObjectIDGenerator) NewID() model.AggregateID {
	return model.AggregateID(primitive.NewObjectID().Hex())
}

type UUIDGenerator struct{}

// NewID genera un UUID v4 y lo retorna como model.AggregateID.
func (g *UUIDGenerator) NewID() model.AggregateID {
	return model.AggregateID(uuid.New().String())
}

// NewObjectIDGenerator crea una nueva instancia de ObjectIDGenerator.
func NewIDGenerator(genType string) ports.IDGenerator {
	switch genType {
	case "uuid":
		return &UUIDGenerator{}
	default:
		return &ObjectIDGenerator{}
	}
}
