// Package repository is the data-access layer. It is the Go equivalent of
// app/repository/*.py. BaseRepository provides the generic CRUD operations that
// every entity repository embeds.
package repository

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/myselfajp/BaseGoAPI/internal/apperror"
)

// BaseRepository provides generic CRUD operations for a model type T.
type BaseRepository[T any] struct {
	db           *gorm.DB
	resourceName string
}

// NewBaseRepository builds a BaseRepository for a model with a human-readable
// resource name used in error messages.
func NewBaseRepository[T any](db *gorm.DB, resourceName string) *BaseRepository[T] {
	return &BaseRepository[T]{db: db, resourceName: resourceName}
}

// DB exposes the underlying handle for repositories that need custom queries.
func (r *BaseRepository[T]) DB() *gorm.DB { return r.db }

// Get returns the entity with the given primary key, or (nil, nil) when it does
// not exist. The condition is written explicitly so it is safe for both numeric
// and string (UUID) primary keys — GORM's inline-condition shortcut treats a
// non-numeric string as raw SQL, which we must avoid.
func (r *BaseRepository[T]) Get(id any) (*T, error) {
	var entity T
	err := r.db.Where("id = ?", id).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

// List returns a page of entities ordered by primary key.
func (r *BaseRepository[T]) List(limit, offset int) ([]T, error) {
	if limit < 0 || offset < 0 {
		return nil, apperror.BadRequest("limit/offset must be >= 0")
	}
	var entities []T
	if err := r.db.Limit(limit).Offset(offset).Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

// Create inserts a new entity, translating constraint violations into a 409.
func (r *BaseRepository[T]) Create(entity *T) error {
	if err := r.db.Create(entity).Error; err != nil {
		return r.mapWriteError(err)
	}
	return nil
}

// Save persists all fields of an existing entity.
func (r *BaseRepository[T]) Save(entity *T) error {
	if err := r.db.Save(entity).Error; err != nil {
		return r.mapWriteError(err)
	}
	return nil
}

// Delete removes an entity.
func (r *BaseRepository[T]) Delete(entity *T) error {
	if err := r.db.Delete(entity).Error; err != nil {
		if errors.Is(err, gorm.ErrForeignKeyViolated) {
			return apperror.Conflict(
				fmt.Sprintf("Cannot delete %s: referenced by other records", r.resourceName),
			)
		}
		return err
	}
	return nil
}

// mapWriteError converts database constraint errors into client-facing
// AppErrors, mirroring the IntegrityError handling in the FastAPI base repo.
func (r *BaseRepository[T]) mapWriteError(err error) error {
	switch {
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return apperror.Conflict(
			fmt.Sprintf("%s already exists or violates a constraint", r.resourceName),
		)
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		return apperror.Conflict(
			fmt.Sprintf("%s violates a foreign key constraint", r.resourceName),
		)
	default:
		return err
	}
}
