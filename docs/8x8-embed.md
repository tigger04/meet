sanitize: defaulting to oed + symbols
<!-- Version: 0.1 | Last updated: 2026-04-18 -->

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

See `docs/jwt.md` (if created) or the 8x8 developer portal at
`developer.8x8.com/jaas/docs/api-keys-jwt` for the full JWT claim structure.
1 -ize correction
4 symbol replacements
