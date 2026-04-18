<!-- Version: 0.2 | Last updated: 2026-04-18 -->

# 8x8 JaaS Embed Reference

## JitsiMeetExternalAPI

The meeting UI is rendered client-side via the `JitsiMeetExternalAPI` constructor.
The main options passed are:

- `roomName` - `{app-id}/{room-slug}`
- `parentNode` - DOM element to render into
- `jwt` - optional RS256 JWT for authenticated/moderator access

## Customization

Most behavioural and UI tweaks are available via the `configOverwrite` object
passed to the constructor:

```javascript
const api = new JitsiMeetExternalAPI("8x8.vc", {
    roomName: "...",
    parentNode: document.querySelector('#jaas-container'),
    configOverwrite: {
        startWithVideoMuted: true,
        disableDeepLinking: true,
        // ... other knobs
    }
});
```

The full list of available config options is documented in the Jitsi Meet
`config.js` reference and the 8x8 JaaS developer docs.

## Events and Commands

Post-construction behaviour is controlled via `addEventListener` and
`executeCommand`. Current usage:

- `videoConferenceJoined` - fires when a participant enters the room.
  Used to set tile view on join: `api.executeCommand('setTileView', true)`.

## JWT Structure

The moderator JWT includes:

- `context.user.moderator: "true"` - grants moderator privileges
- `context.features.recording: true` - enables recording capability

See the 8x8 developer portal at
`developer.8x8.com/jaas/docs/api-keys-jwt` for the full JWT claim structure.

## Recordings

Recording is enabled via the `features.recording` flag in the moderator JWT.
After a session ends, 8x8 delivers the recording via webhooks:

1. `RECORDING_ENDED` - signals recording stopped (no download URL)
2. `RECORDING_UPLOADED` - contains `preAuthenticatedLink` (valid 24 hours)

Automated download is tracked in [#1](https://github.com/tigger04/meet/issues/1).

See `developer.8x8.com/jaas/docs/webhooks-payload/` for payload details.
