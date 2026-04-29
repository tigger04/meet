<!-- Version: 0.3 | Last updated: 2026-04-29 -->

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
- `recordingStatusChanged` - fires when recording starts or stops.
  Used to sync the banner record button state.

## Banner Record Button

Moderators (identified by presence of a JWT query parameter) see a
Record/Stop button in the banner. The button:

- Calls `api.executeCommand('startRecording', { mode: 'file' })` to start
- Calls `api.executeCommand('stopRecording', { mode: 'file' })` to stop
- Listens to `recordingStatusChanged` to stay in sync with the actual
  recording state (e.g. if stopped via the Jitsi UI instead)

Regular visitors never see the button.

## JWT Structure

The moderator JWT includes:

- `context.user.moderator: "true"` - grants moderator privileges
- `context.features.recording: true` - enables recording capability

See the 8x8 developer portal at
`developer.8x8.com/jaas/docs/api-keys-jwt` for the full JWT claim structure.

## Recordings, Transcriptions, and Chat

Recording is enabled via the `features.recording` flag in the moderator JWT.
After a session ends, 8x8 delivers files via webhooks to
`POST /webhook/recording`:

| Event | Content | Filename pattern |
|-------|---------|-----------------|
| `RECORDING_UPLOADED` | Video recording | `{room}_{date}_{time}_{duration}s.mp4` |
| `TRANSCRIPTION_UPLOADED` | Speaker-attributed transcript | `{room}_{date}_{time}_transcript.{ext}` |
| `CHAT_UPLOADED` | Meeting chat log | `{room}_{date}_{time}_chat.{ext}` |

All three carry a `preAuthenticatedLink` valid for 24 hours. The server
downloads each file and uploads it to Nextcloud via WebDAV automatically.

All authenticated webhook events (any type) are logged via structured JSON
logging for observability.

See `developer.8x8.com/jaas/docs/webhooks-payload/` for payload details.
