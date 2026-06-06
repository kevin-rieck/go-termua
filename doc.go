// Package toast provides transient, non-blocking Toasts for Bubble Tea
// applications using Lip Gloss rendering.
//
// A Model owns the Toast Stack, queued overflow, Dismissal, placement, and
// rendering. Apps normally request Toasts with Show and explicitly route Bubble
// Tea messages through Model.Update. Small apps and tests can use Model.Push
// directly.
package toast
