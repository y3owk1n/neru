// Package hints renders the hints overlay: the labels drawn over clickable
// elements, and the search input drawn beside them.
//
// It owns style and drawing only. Which elements are hinted, what the labels
// are and what a keystroke narrows them to live in internal/domain/hint, and
// the state one hints session keeps lives in internal/app/components/hints.
package hints
