package ipc

import "github.com/y3owk1n/neru/client"

// The reply types live in the public client package, so a Go program reads
// the daemon's replies with the types the daemon encodes them from.

// StatusData is client.StatusData.
type StatusData = client.StatusData

// CursorSlotData is client.CursorSlotData.
type CursorSlotData = client.CursorSlotData

// ScrollInvertData is client.ScrollInvertData.
type ScrollInvertData = client.ScrollInvertData

// ScreenShareData is client.ScreenShareData.
type ScreenShareData = client.ScreenShareData

// FollowSelectionData is client.FollowSelectionData.
type FollowSelectionData = client.FollowSelectionData

// WatchLine is client.WatchLine.
type WatchLine = client.WatchLine

// DisplayData is client.DisplayData.
type DisplayData = client.DisplayData

// DisplaysData is client.DisplaysData.
type DisplaysData = client.DisplaysData

// CursorData is client.CursorData.
type CursorData = client.CursorData

// WindowData is client.WindowData.
type WindowData = client.WindowData

// AppData is client.AppData.
type AppData = client.AppData

// HintData is client.HintData.
type HintData = client.HintData

// HintsData is client.HintsData.
type HintsData = client.HintsData
