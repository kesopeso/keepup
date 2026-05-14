# KeepUp Backlog

## High Priority

- Current-viewer stale recovery prompt
  - Show when the authenticated viewer loads an active route while their persisted status is `stale`
  - Actions: resume sharing or continue as spectator
  - Automatic recovery still happens when accepted positions resume during an existing live session
- Optional path snapping / map matching
  - Keep raw accepted points as source of truth
  - Generate snapped geometry as derived data
  - Support transport-aware behavior later
  - Preserve raw vs snapped comparison for debugging/replay

## Planned Later

- Live chat for route members
- Replay mode for closed routes
  - chronological playback
  - playback speeds: `1x`, `2x`, `4x`, `8x`, `16x`
  - replay route movement and chat together
- Timeline UI for route history
- Member transport mode changes mid-route
  - persist transport changes as timestamped events
  - reflect changes in live state and future replay
- Owner-granted tracking permissions on restricted routes
  - selectively allow specific members to start sharing
  - allow permission changes during active route
  - persist permission changes as route events
- Hide/filter left members in the member list
- Light mode / theme switching
- Route code regeneration or archive access rotation
- Route/archive expiry policies
- Route snapshot chunking or pagination for very large histories
- Redis-backed presence/pubsub if horizontal scale requires it
