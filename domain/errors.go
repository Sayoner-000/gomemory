package domain

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrValidation    = errors.New("validation error")
	ErrAlreadyExists = errors.New("already exists")
	// ErrCriticalContextOverflow indica que el conjunto de ContextItem con
	// Priority=PriorityCritical excede por sí solo el presupuesto de tokens
	// solicitado. BuildContextPack nunca degrada esto a un ContextPack
	// parcial: o todo lo crítico entra, o no se devuelve paquete (feature 015).
	ErrCriticalContextOverflow = errors.New("critical context exceeds configured token budget")
	// ErrInvalidContextRequest indica que un ContextRequest llegó con
	// Task/Project vacíos o MaxTokens<=0 — se valida en el borde, antes de
	// tocar cualquier puerto (feature 015).
	ErrInvalidContextRequest = errors.New("invalid context request")
)
