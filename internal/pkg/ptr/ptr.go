// Package ptr содержит тривиальные хелперы для работы с указателями.
//
// Цель — сократить запись `s := "x"; &s` в одном месте.
package ptr

// To возвращает указатель на переданное значение.
//
//	ptr.To(42)        → *int
//	ptr.To("hello")   → *string
//	ptr.To(time.Now())→ *time.Time
func To[T any](v T) *T { return &v }

// Deref возвращает значение по указателю; для nil возвращает zero-value.
// Удобно, когда хочется fallback на ноль, а не if/else.
func Deref[T any](p *T) T {
	if p == nil {
		var z T
		return z
	}
	return *p
}

// DerefOr возвращает значение по указателю или fallback для nil.
func DerefOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}
