# Alisten UI Design and Engineering Guidelines

This document describes the page structure, visual parameters, interactions, and performance constraints of the current Native Shell (NSH) implementation. Update the relevant sections whenever the implementation changes so that the documentation stays aligned with the code.

## 1. Product and Design Direction

Alisten is a real-time shared listening application. The interface centers on listening, searching and requesting songs, the playback queue, and room conversations.

The current design takes its top search bar, independent content panels, and bottom player layout from Spotify references while retaining the brand green accent (`#68D268`). Lyrics are the visual focus of the listening page; avoid large album information blocks, recommendation cards, and dashboard-style modules.

- Do not display a left sidebar. Place page navigation in the top More menu.
- The top bar and bottom player use solid black `#000000`, with no outer borders, corner radii, outer margins, or backdrop blur.
- Distinguish the main content and right panel through background colors, spacing, and rounded corners, without outer borders.
- Use flat neutral colors for large areas. Do not add continuous background animations, purple glows, or decorative color gradients.
- Search is the entry point for requesting songs. Do not add a separate song-request navigation item or right-panel tab.

## 2. Desktop Layout

The application fills the viewport. Use Grid for the outer layout and Flex within the workspace. Page navigation must keep the bottom player and audio layer mounted.

| Area | Current dimensions and behavior |
| --- | --- |
| Top bar | Height 76px, horizontal padding 24px, horizontal gap 20px |
| Workspace | Fills the remaining height, outer padding 8px, gap between main content and right panel 8px |
| Main content panel | Fills the remaining width, corner radius 12px, no outer border |
| Optional right panel | Width 480px, does not shrink, corner radius 12px, no outer border |
| Bottom player | Dedicated layout row 104px high, padding 12px 16px, horizontal gap 16px |
| Standard page content | Maximum width 1400px, centered, padding 32px 40px 48px |
| Lyrics page | Fills the main panel height, padding 24px 32px, without the standard page's maximum-width wrapper |

Flexible areas in layouts with multiple columns must set `min-width: 0` and `min-height: 0`. The right panel must not cover the player. Recenter the lyrics for the new container dimensions whenever the panel opens or closes.

### Top Bar and Navigation

From left to right: More, Back, and Forward navigation buttons; Home and the search field; connection status and the user avatar. Back and Forward navigate the application's page history and are disabled at its boundaries.

- Home returns to the listening page after joining a room, or opens room discovery before joining.
- While on the home page, keep the top Home icon highlighted in brand green and expose its selected accessibility state. Restore its normal color on other pages.
- Connection status indicates not joined, connecting, synchronized online, or reconnecting. After joining, a disconnected status can be clicked to retry.
- The More menu contains Listening, Playlist, Room Chat, Playback History, Discover Rooms, Settings, Share Room, and Help.
- Place Home immediately to the left of search. Share Room is available only in the More menu. Chat and playlist shortcuts belong in the bottom player; the avatar opens personal settings.
- Use an anchored NSH `Overlay` for the More menu. Close it after selecting an item, pressing outside, or pressing Escape / Back.
- The top bar's solid black styling applies to the bar itself. The search field, popover, and menu retain their own shapes and surface hierarchy.

### Search Entry Point

The search field is flexibly centered, with a maximum width of 620px, height 48px, and corner radius 24px. Its background is `#1F1F1F` both at rest and when expanded. The placeholder asks what the user wants to play.

1. Clicking or focusing the field opens a recent-search popover only when search history exists and recording is enabled. Do not show platform or type options, an explanatory heading, a close button, footer hints, or a search button. Close it on an outside click or Escape.
2. Entering a query and pressing Enter closes the popover and opens a dedicated results page.
3. On the results page, select NetEase Cloud Music or QQ Music, and All, Songs, or Playlists, while retaining the query.

When history is enabled, the popover shows up to 4 recent searches. Clicking an entry searches immediately. Do not send music search requests before connecting to a room. Playlist queries beginning with `*`, including those with leading spaces, must not enter search history; this also applies when opening playlists or changing pages. Remove existing records of this kind on startup. Preserve the existing behavior of ordinary searches and `@` user queries.

Never expose the `*id` query form in the interface. Opening a playlist keeps sending `*id` to the server and shows the playlist name in the results heading only — the search fields (top bar and the mobile in-page field) keep the keyword the user typed, because opening a result is not a new search. Switching to Songs or All keeps showing that playlist's songs; switching to Playlists searches that keyword, because the `*id` form matches nothing there. Typing and submitting a new query replaces the field text as usual.

Deduplicate search history by query, ignoring leading and trailing whitespace and letter case. Keep only one entry for a query across platforms and categories. Update it to the most recently used platform and category and move it to the front. On startup, merge older duplicates, retain the newest record, and update local storage.

The results page uses the shared top search bar without another input. Provide All, Songs, Playlists, and Personalized Recommendations filter pills and a music platform selector. Switching categories or platforms retains the query and synchronizes the top search bar. Highlight the active filter with a light fill. Keep the category bar sticky as the main content scrolls.

All searches songs and playlists in parallel. Show the top match and a preview of 4 songs side by side, followed by 5 playlists below. Show All opens the corresponding category. The Songs category uses title, album, and duration columns. Playlists use a cover grid with exactly 5 columns on a standard desktop, 3 when the content width is at most 720px, and 2 when it is at most 480px. Category pages retain pagination, loading, empty states, retry actions, and song requests. If one query category fails, continue displaying the successful results of the other.

Show an opening arrow on playlist hover or focus. Songs retain a separate circle-plus request button and a music-source link on the cover. Clearing the top search shows recent searches. Personalized recommendations stay within the search page, suggest similar songs from the room's playback history, and leave the listening home page dedicated to lyrics. On mobile, state that basis in a hint below the page heading with a brand-green sparkle icon; keep the hint hidden on desktop. Do not provide an audio-URL request form on the search page. All layouts respond to the search container's width and remain usable alongside the right chat or queue panel.

This redesign evolves the existing brand with `DESIGN_VARIANCE=4`, `MOTION_INTENSITY=2`, and `VISUAL_DENSITY=6`. Retain system fonts, brand green, and 8 / 12px corner radii, without adding dependencies or mock music data. Category organization references [Spotify's search guidance](https://support.spotify.com/us/article/search/); preserve this project's synchronized rooms and song-request behavior.

### Room Discovery and Joining

Room discovery is the joining entry point before a room is entered. Present it as a Spotify-style card shelf instead of a table or a plain list.

- The page header keeps the brand eyebrow, the `text-ui-xl` title, and one supporting line. Do not add statistics, banners, or illustration blocks.
- The toolbar holds one pill-shaped room search field (48px desktop, 44px mobile, radius 999px) and the brand-green Create Room action. Creating a room opens the shared dialog form.
- Each room is one card button: an `#181818` surface through the page-local `--color-room-card`, 12px padding, a 12px radius, and a 12px gap between cover and copy. Hover and keyboard focus lighten the card to `--color-surface` over 150ms. Cards carry no border.
- Inside the card: a cover area with an 8px radius and a brand-green headphones glyph, the room name as a single-line `text-ui-base` title, a two-line clamped `text-ui-caption` description, and a `text-ui-sm` member count.
- A locked room shows a pill badge with a lock icon and `密码` at the cover's top-left. Do not place the lock beside the room name.
- Hover or keyboard focus reveals a 40px brand-green circular join affordance (34px on mobile) at the cover's bottom-right, fading in and sliding up over 150ms. Devices without hover show it permanently. It is decorative: the whole card opens the join dialog, and it is never a nested button.
- Loading uses card-shaped placeholders in the same grid, not row skeletons.
- The join dialog opens with a compact header: an 88px square cover beside the room name, the description, and the member count with a lock note for locked rooms (72px on mobile). Show the password field only for locked rooms and span the primary action across the dialog as a pill.

### Right Panel

The panel is closed by default when joining a room. On desktop, open chat or the queue from the bottom player; clicking the same entry again closes it. If that content is already displayed as the main page, clicking again returns to the listening page.

Room joining keeps the password dialog visible while connecting, disables duplicate submissions, and enters the listening page only after the WebSocket opens. A failed initial join stays in the dialog with a retryable error and does not save the rejected password. Closing a pending join cancels that attempt; established room connections retain unlimited reconnects.

Keep the room password input, validation error, and Join action together in a fixed dialog footer outside the scrolling room information. Reserve the measured header and footer heights when sizing the scroll area, so opening the keyboard does not require scrolling to reach the input or action. Focus the password field after the dialog finishes appearing, avoiding keyboard reveal calculations during the entrance animation. Apply the bottom safe area once, and omit its extra spacing while the keyboard is visible.

Start clock calibration immediately on connection. Until the first measured clock sample arrives, use the snapshot's server timestamp to seed the offset before applying playback. Wait for the current audio source's load event before seeking to the room's current position and starting playback; compute that position at load completion so URL resolution and buffering time are included. Reconnect snapshots for an unchanged song reuse the loaded source and explicitly resynchronize, without waiting for another load event or reloading the audio. Ignore load events from replaced sources, and allow a failed source to be recreated even when resolving it returns the same URL.

Opening and closing use a 240ms animation of width, a slight horizontal offset to the right, and opacity. Keep the inner content at a fixed width while the lyrics area adapts to the available width. Disable interaction during closing and unmount the content after the animation. Low performance mode retains only a 120ms fade; off and reduced motion switch immediately.

Switching between chat and the playlist fades only the inner panel content: fade out over the first 30% of a 220ms transition, replace the view, then ease out to full opacity. Keep the panel shell, width, main content, and player still; do not run layout or slide animations when switching between chat and the playlist. Queue and Recently Played use a shared 280ms horizontal slide: a brand-green underline matches the measured label width and smoothly changes its position and width between tabs while both content pages slide together in the corresponding direction. Keep the tab header fixed, clip the pages inside the panel, preserve each page's scroll position, and allow a new click to reverse the in-progress animation. Low performance mode shortens the slide and chat fade to 160ms; off and reduced motion switch instantly. Opening or closing the panel from closed plays only the panel's own opening/closing animation. Rapid switches cancel the previous transition so an older request cannot replace the latest view. Other page switches only fade, except the mobile bottom tabs described below.

On mobile, switching between Listening, Queue, and Chat slides the content horizontally over 300ms in tab order. A tab to the right brings content in from the right as the previous page leaves to the left; reverse this when selecting a tab to the left. Keep the header, player, and bottom navigation stationary and clip the pages inside the content area. Use one native animated position and retarget it on rapid taps. Retain outgoing and crossed pages only during the transition, then release inactive pages. Inactive pages cannot receive touches or accessibility focus. Low performance mode shortens the slide to 160ms; off and reduced motion switch immediately. Back/Forward navigation between these tabs follows the same direction.

- The queue and recently played panel has Queue, Recently Played, and Close controls at the top, without a song-request tab.
- Open chat through its own shortcut and reuse the chat component. Do not show queue or recently played tabs above it. A single title row contains Room Chat, the online member count, and the panel close button.
- The member count opens a non-modal popover anchored under that button: a popover surface with a 12px radius and a header row carrying the count, then a scrollable list of 28px round avatars and names that truncate. It overlays the message list instead of pushing it down, and closes on an outside click, Escape, or a second press of the button. Cap it at 272px wide and 280px tall so it fits the desktop panel and a narrow phone.
- Chat messages use two columns for avatar and content, with nickname and time adjacent. Truncate long nicknames and wrap long messages. Keep the title, bottom composer, and the member popover overlay outside the message scroll area. Place Send inside the composer. The full page and panel share this layout.
- The queue shows the current and upcoming songs as compact rows, hiding secondary columns such as index, album, and duration.
- Both the full playlist page and queue panel hide scrollbars while retaining wheel, touch, and keyboard scrolling.
- Recently Played uses a compact history view without the full page title, filters, or per-row timestamps. It scrolls independently within the remaining panel height while the top tabs stay fixed. Both the panel and full history page hide scrollbars while retaining wheel, touch, and keyboard scrolling.
- Recently played song information is plain text, with separate request and delete buttons on the right. Use circle-plus icons for requests in recently played, search results, and recommendations. On hover or press, request, like, and delete buttons change only their icon color to brand green, without added backgrounds, outlines, or scaling. Preserve visible keyboard focus. Disable buttons and show an ellipsis during asynchronous operations; show feedback on success or failure.
- Do not duplicate the same view in the panel and main page at the same time.

## 3. Lyrics Main View

The listening room's main view displays synchronized lyrics only. The bottom player carries the song title, artist, and cover, without repeating them in the main panel.

Center lyrics horizontally and keep the current line at the vertical center of the scroll container. Each line has a maximum layout width of 880px, horizontal padding 32px, vertical padding 8px, line height 1.5, and font weight 600. Allow wrapping without truncating lyrics.

### Size and Highlight Hierarchy

All lyrics use `text-ui-xl` as the base font size. Create the hierarchy with `transform` scaling; do not animate `font-size`.

| State | Scale | Vertical offset | Opacity | Text color |
| --- | --- | --- | --- | --- |
| Current line | 1.18 | 0 | 1 | `--color-foreground` |
| Immediately preceding and following lines | 0.8 | 2px | 0.9 | `--color-foreground-subtle` |
| Other lines | 0.7 | 4px | 0.78 | `--color-foreground-subtle` |

With the default 28px base size, the apparent sizes are approximately 33 / 22 / 20px. The current line is about 48% larger than adjacent lines; keep that distinction clear. Maintain compact spacing between lines without restoring large vertical gaps.

### Transitions and Scrolling

- Scale, translation, color, and opacity share one native `AnimatedValue` across all lyric rows, with per-row start/end interpolation and a 420ms native timing curve. No JavaScript runs per animation frame.
- First alignment sets the shared value immediately; subsequent index changes animate only the affected rows.
- After the lyric index changes, the Store coalesces centering scroll updates with rAF. Scroll smoothly in normal mode; position immediately on first alignment and container size changes.
- On Web, lyric emphasis uses CSS transitions. Android and other native platforms use a shared animated value whose graph plays on the UI thread, without a JavaScript frame loop. Low performance mode animates only opacity; off and reduced motion switch immediately.
- Android defaults to balanced performance so native lyric scaling and smooth centering are enabled on first launch; saved user choices still take precedence. Repeated layout callbacks with unchanged dimensions must not interrupt a smooth lyric scroll with immediate recentering.
- Use NSH `onLayout` measurements to handle panel toggles, window resizing, and wrapping-induced position changes.
- Hide scrollbars with `scrollbar={false}`. Use static, pointer-transparent gradient overlays at the top and bottom instead of an animated full-viewport mask; the fully visible region begins at 12% and ends at 88%.
- Expose the current line through its selected accessibility state. Show an icon, explanation, and empty state when lyrics are unavailable.
- Blank timestamp entries participate only in synchronization; they create no native text views and occupy no line height. During blank sections, remove the current-line highlight, keep the preceding and following lines at the slightly enlarged adjacent-line scale, and center automatically at the boundary. Reserve centering scroll space only at the beginning and end of the container.
- Update styles and accessibility state through targeted signal bindings instead of reconstructing the lyric tree on every progress update.
- Follow the performance guidelines for low, off, and reduced motion. Preserve the size and highlight hierarchy when animations are disabled.

## 4. Bottom Player

Mobile reference update: bottom navigation is Listening / Search / Queue / Chat, with directional transitions in that order. The top Home and Search shortcuts are desktop-only; on mobile retain More, Back, room identity/connection status, and profile. Reserve equal space on the left and right so the room name centers relative to the screen, truncating long names. The mini-player uses a 56px cover, a larger song title (base font + 4px), 12px corners, 8px side margins, a content-driven minimum height of 82px, and a solid muted blue-green #203d43 fill while a song is present (#24282a when empty). Hide elapsed/total time on mobile; keep the 3px progress line spanning the card's bottom edge. Vote to Skip is an 18px bare icon without a circle, border, or background, retaining a 44px touch target. Do not add headphones, Bluetooth/device controls, or local playback buttons. These mobile rules supersede the earlier mini-player dimensions below; desktop styling remains unchanged.

Use a horizontal layout: cover on the left, song information and a wide progress bar in the middle, and skip voting, volume, and auxiliary shortcuts on the right.

- Fix the background at `#000000`, flush with the outer frame, without borders, corner radii, shadows, or blur.
- Desktop cover size is 64px with a 12px corner radius. Clicking the cover returns to the lyrics page.
- Use `text-ui-lg` for the song title and `text-ui-caption` for the artist. Truncate long text to one line.
- The middle song information is plain, non-clickable text without button states.
- Place the progress bar below the song information with a 10px top gap and a desktop height of 3px. Align time to the right as elapsed time / total duration.
- Progress is display-only, without drag-to-seek. Local progress controls must not change the room's synchronized playback position.
- The only playback control is Vote to Skip. Do not provide Previous or local Play / Pause buttons. Disable skip when there is no song, no connection, or an ongoing request.
- The volume area contains Mute, a 0–100 slider, and chat and queue shortcuts. The desktop slider is 90px wide.
- Bottom chat and queue buttons turn brand green on hover, press, and keyboard focus. Keep the corresponding button highlighted and set `aria-pressed` while its main page or panel is open; restore its normal color after closing. Disable these buttons before joining a room.
- Playback synchronizes automatically with the room, without a manual sync button. Settings in the More menu opens the application's settings page.
- NetEase Cloud Music Cookie settings are hidden by default. Reveal and scroll to them after 7 consecutive activations of the settings page title, with no more than 2 seconds between adjacent activations. Support keyboard activation too. Unlocking lasts only for the current settings page visit and is not persisted; leaving and returning requires unlocking again.
- With no song, display a music placeholder icon, no-song and unknown-artist labels, and zero progress.
- On mobile the player is a floating card with 8px horizontal margins, a 12px radius, and a content-driven minimum height of 68px. Use 6px vertical and 8px horizontal inner padding. Keep the 56px cover, 18px skip icon, and 44px button target unchanged; reduce the card's surrounding space rather than scaling its contents. Show the title, artist, and a 3px bottom progress line; hide timestamps, volume, and auxiliary shortcuts.

## 5. Colors, Themes, and Surfaces

Default to dark mode, with light and system-following options. Switch themes through root `data-theme` and semantic CSS variables, persisting the choice in appearance settings.

The top bar, its menus and search popover, and the bottom player retain local dark variables, so the top and bottom bars remain black with light text in light mode. The main content, right panel, and settings page backgrounds always use `#121212`, with local dark variables for readable text and controls. Settings groups use `#1F1F1F` backgrounds. Standalone dialogs continue to follow the selected theme.

| Purpose / Token | Dark | Light |
| --- | --- | --- |
| Frame `--color-frame` | `#000000` | `#e8e8ed` |
| Main panel `--color-content` | `#121212` | `#121212` |
| Right panel `--color-panel` | `#121212` | `#121212` |
| General background `--color-background` | `#1c1c1e` | `#ffffff` |
| Surface `--color-surface` | `#232326` | `#f3f3f6` |
| Popover `--color-popover` | `#242427` | `#ffffff` |
| Input `--color-input` | `#29292d` | `#f3f3f6` |
| Primary text `--color-foreground` | `#f4f4f5` | `#202024` |
| Secondary text `--color-foreground-subtle` | `#a6a6af` | `#64646e` |
| Supporting text `--color-foreground-subtlest` | `#90909a` | `#71717b` |
| Brand accent `--color-brand` | `#68d268` | `#68d268` |
| Primary action `--color-primary` | `#68d268` | `#68d268` |
| Divider `--color-border` | `#333337` | `#e6e6eb` |
| Input border `--color-input-border` | `#414148` | `#d4d4dc` |
| Selection `--color-selected` | `#3b3b3e` | `#e4e4e8` |
| Subtle accent `--color-accent` | `#203320` | `#e7f6e7` |
| Toast `--color-toast` | `#303034` | `#ffffff` |

- Treat the semantic constants and `palette()` in `src/theme.ts` as authoritative. The CSS variable names below remain the design-token names; NSH applies them through typed styles rather than a global stylesheet.
- Standard components use semantic tokens. Do not add ad hoc colors or legacy Tailwind blue and purple colors as new brand accents.
- Card surfaces on the discovery pages use the shared `card` token, fixed at `#181818`, because the main content panel keeps the dark palette in every theme.
- Success, warning, and error colors communicate actual states only, without decorative use.
- The no-border rule applies to the main content, right panel, and top and bottom bars. Inputs, popovers, dialogs, cards, and subtle internal dividers retain necessary boundaries; define them with surface color before reaching for a border.
- Do not layer backdrop blur onto large panels, the lyrics container, or every chat bubble. Covers and floating surfaces may use local shadows.

## 6. Typography, Controls, and Content Density

Use the platform system font without blocking rendering on remote fonts. Icons are bundled SVG assets rendered by `src/components/icon.tsx`. Standard icon buttons use 20px icons with a 1.7 stroke width.

The launcher icon is `icon.png`: a spring-green circular plate on a near-black square, containing two connected black musical notes with a green play cutout in the left notehead. The paired notes express listening together. Keep the mark free of text and fine details for small-size recognition. Web installation icons and the Android launcher icon are generated from this shared source; the image-generation prompt and visual reference are recorded in `icon.prompt.txt`.

UI typography uses the `font(offset)` / `text(offset)` helpers and the `text-ui-*` names below as design tokens, not CSS classes. The user setting changes only the shared UI font signal (12–18px, default 14px), never the platform root font size. Spacing, icons, and layout dimensions must not scale with this setting.

| Token | Formula | Default size | Usage |
| --- | --- | --- | --- |
| `text-ui-xl` | Base + 14px | 28px | Page titles, lyric base size |
| `text-ui-lg` | Base + 4px | 18px | Section titles, dialog titles, bottom player song title |
| `text-ui-base` | Base | 14px | Body text, navigation, buttons, song titles |
| `text-ui-caption` | Base − 1px | 13px | Compact descriptions, bottom player artist |
| `text-ui-sm` | Base − 2px | 12px | Secondary metadata |
| `text-ui-xs` | Base − 4px | 10px | Times, counts, low-emphasis metadata |

Editable mobile controls use the fixed `text-mobile-input-safe` size (16px) to prevent iOS zoom on focus. Do not use it as a general body-text token. Use tabular numerals for times.

- Common spacing values are 4, 8, 12, 16, 20, 24, and 32px.
- Standard buttons, inputs, and covers commonly use an 8px radius. Icon buttons default to 36 × 36px.
- The main content, right panel, room cards, top-level playlist covers, and search popover use a 12px radius; covers nested in a padded card use 8px.
- Dialogs and toasts use a 16px radius. The search field is pill-shaped; avatars and Home buttons are circular.
- Playlist and recently played rows share a base layout: minimum height 68px, cover 42px, vertical padding 10px, horizontal padding 8px, and a flexible information area. Vertically center covers, text, and action buttons. Retain the same horizontal padding in the panel and on mobile.
- Covers in song lists, the queue's current song, and playback history link to the music source when a valid `webUrl` exists, opening in a new tab. Without a valid HTTP(S) source link, show a plain cover. Show a brand-green external-link icon on hover or keyboard focus. Song requests retain a separate button.
- The desktop room grid creates columns automatically with a minimum width of 235px and a cover aspect ratio of 1.6. Room cards keep a 24px row gap and a 20px column gap. Playlist grid columns have a minimum width of 150px.
- Chat uses avatars, nicknames, times, and bubbles. Allow long messages to wrap. Bubbles follow the selected theme consistently.

## 7. Dialogs, Notifications, and Feedback

- Generic dialogs use native `dialog.showModal()` for an inert background, focus containment, and title association.
- Standard dialog width is `min(480px, 100vw − 32px)`; wide dialogs use `min(760px, 100vw − 48px)`. Long content scrolls within the dialog while the title bar stays visible.
- Escape, the close button, a backdrop click, or browser Back closes the current dialog. Restore the previous focus after closing.
- Android dialogs subtract the reactive `keyboardFrame().height` from the available height and lift bottom sheets above the keyboard. Their scroll areas use `keyboardShouldPersistTaps="handled"` so actions respond on the first tap while editing.
- At 767px and below, dialogs become bottom sheets: full viewport width, anchored to the bottom edge, a 16px top radius and square bottom corners, and a 240ms upward slide that low performance mode shortens to 120ms. Keep the title bar sticky, keep at least 16px of viewport height above the sheet, and add the bottom safe-area inset to the body padding. Sheet actions span the sheet width.
- The top search popover is non-modal. Do not apply native modal focus trapping or an inert background.
- Dropdown surfaces — the More menu, the search history popover, and the member popover — enter with a 200ms fade plus an 8px drop and a 0.96 scale from their anchor edge (`transform-origin` at the anchor corner). Because the native details slot keeps its content rendered while closed, key the More menu's animation off `[open]` so it replays on every open. Low performance mode keeps the 160ms fade without the transform; off and reduced motion switch instantly.
- Settings is an application page within the main content area. Retain top navigation, the bottom player, and mobile navigation, without a native dialog, backdrop, or focus trap. On desktop, it can appear beside chat, the queue, or recently played. Entering settings preserves the open panel; bottom shortcuts only toggle or switch panels without leaving settings or discarding drafts. Keep the settings title and bottom save area fixed while the content scrolls independently. Do not provide a close button. Top Back returns to the previous page, and Forward can reopen settings.
- Profile, appearance, playback, performance, history, and Cookie edits remain local page drafts until the shared bottom Save button updates Stores, local storage, and the server. Leaving the page discards unsaved drafts. Remove individual section save buttons. Cookie status queries only read status and do not save the token. Disable controls and duplicate submission while saving; retain failed items as drafts for retry.
- Settings uses a single column of sections with rounded groups below section titles and fine internal dividers. Each row places its label and description on the left and control on the right; boolean options use switches. On mobile, place inputs and volume controls below descriptions. Preserve theme colors and brand-green accents.
- Settings responds to its own container width. At widths of at most 640px, reduce padding and move inputs and volume controls below descriptions. Apply the same narrow layout when a desktop right panel is open. Keep the save area visible.
- On narrow settings pages, stack every value control (including selects) below its label and description; keep only switches beside the text. Read the current container width reactively rather than capturing the first layout. Use a full-width Save button below the status message on mobile. Retain `#121212` for the page and `#1F1F1F` for setting groups.
- Settings selects and the music-source picker share an anchored overlay. Keep it beside the trigger, flip it when needed, and constrain it to the viewport; do not use a native bottom selection sheet. Mobile triggers are at least 48px tall and option rows 60px tall with larger text, brand-green selection and a check mark. Long menus scroll internally. Outside press, Escape and Android Back close the overlay.
- Center toasts below the top search bar, 88px from the viewport top on desktop and 76px on mobile, plus the top safe-area inset. Maximum width is 420px. Stack them from top to bottom in creation order. An oversized stack scrolls internally with hidden scrollbars. The search popover sits above notifications.
- Asynchronous pages provide static skeletons, empty states, error messages, and retry actions as appropriate. Do not substitute continuous flashing for clear loading feedback.
- Every icon action has a clear `aria-label`; inputs have labels and keyboard focus is visible. Do not communicate state through color alone.
- Target WCAG AA text contrast. Check long song titles, nicknames, multiple languages, and the largest font setting.

## 8. Responsive Layout and Scrolling

Read responsive widths inside reactive bindings so resizing and rotation update the main shell, player, lists, lyrics, dialogs and notifications. Mobile icon and text actions have at least 44px targets; inputs can shrink within rows and use at least 16px text. Search categories scroll horizontally inside their own strip. History controls wrap into separate rows on narrow screens, chat names truncate before timestamps, and the mini-player places time below song information instead of reserving part of the title line for it. Keep icons from shrinking and include the top safe area when placing notifications.

| Viewport | Current adjustments |
| --- | --- |
| Above 1100px | Standard desktop layout, right panel 480px, standard content padding 32px 40px 48px |
| 768–1100px | Right panel 420px, standard content padding 24px; hide brand text and the song album column; bottom volume slider 60px wide |
| At most 767px | Hide the right panel and switch main pages; top bar 64px, a content-sized floating mini-player, navigation 60px plus the bottom safe-area inset |

Mobile details:

- Retain More, Back, Forward, Home, Search, and the avatar at the top on desktop. Hide connection status and tighten control spacing on narrow screens.
- At 767px and below, drop Forward and the inline search field: the bar keeps More, Back, Home, a search icon button, and the avatar. The search icon opens the search page, whose own field sits in the page body. Hide the connection status.
- The results page keeps its category chips and platform selector in a sticky bar; on mobile the in-page search field joins that bar as its first row, so the field and chips stay pinned while results scroll. Below 767px an empty query shows recent searches or the empty state, without extra browse shelves.
- Search height is 42px. Hide type shortcut text. Fix the popover below the top bar with 12px horizontal margins.
- Remove the main content panel's corner radius and outer spacing. Standard content padding is 24px 16px 32px; lyrics page padding is 16px.
- Bottom cover size is 44px with a 6px radius inside the mini-player card; use `text-ui-base` for the song title. On mobile the progress line spans the card's full width along its bottom edge, with the elapsed / total time to the left of the skip button. Progress height is 2px.
- The bottom player retains song information, time, progress, and skip, while hiding volume and auxiliary shortcuts. Volume and mute remain available in settings.
- Bottom navigation contains only Listening, Queue, and Chat in three equal-width items, on a `--color-frame` bar with 24px icons and 10px labels. Access History and Rooms through the top More menu; search remains at the top.
- The mini-player and navigation rows size to their content, so a larger font setting grows them instead of clipping. Dialogs open as bottom sheets.
- Room and playlist grids use two columns, with square room covers. Room cards tighten to 10px padding and a 12px grid gap, the room search field is 44px high, and the join affordance is 34px. Song lists hide secondary information such as index and duration.

Prefer `100dvh` for viewport height and retain `--vh` as a fallback for older browsers. Handle `safe-area-inset-*` in the outer layout, mobile navigation, and dialogs. Prevent html / body scrolling. Allow scrolling only within main content, lyrics, chat, lists, or dialogs, so content cannot push the player off screen.

## 9. Performance and Engineering Constraints

### Rendering and Resources

- Use NSH + TypeScript with native views. Keep external business state in EventTarget Stores without introducing another state framework.
- Stores create new state snapshots. Subscribe through `useStore` and stable field selectors, which bridge snapshots to NSH signals and avoid unrelated native updates.
- Keep high-frequency progress, scrolling, and lyric indices out of component reconstruction. Drive progress `scaleX` with a native `AnimatedValue` updated at most 30 times per second from media progress events; do not run continuous native timing or a JavaScript frame loop. Pause visual updates in the background and recalibrate on seeks, playback/buffering changes, or song changes. Update time text and progress accessibility values only when the whole second changes.
- Locate the current lyric line with binary search and coalesce scrolling with rAF. Use `onLayout` measurements for centering instead of DOM queries or `ResizeObserver`.
- Use bundled SVG icons and NSH's native image decoding/cache; show a music placeholder on failure.
- Render chat and history in batches. Use NSH `List` for new long or unbounded lists instead of creating every row at once.
- Clean up subscriptions, media events, timers, rAF callbacks, and layout registrations. Visual changes must not introduce continuous polling or frequent full-page updates.

### Motion Levels

| Setting | Lyrics and visual behavior |
| --- | --- |
| `high` | Full scale, translation, color, and opacity transitions; smooth lyric centering; permitted local shadows and effects |
| `medium` | Full lyric transitions with platform-appropriate local effects |
| `low` | Immediate size and color changes, retaining only a short opacity transition; immediate lyric centering; remove cover shadows |
| `off` | Disable animations, transitions, smooth scrolling, shadows, filters, and lyric masks; retain static size and highlight hierarchy |
| Reduced motion | Disable transitions, animations, and smooth scrolling when enabled manually or requested by the system through NSH `reduceMotion()` |

The top bar and bottom player never use blur. New effects must cover every performance level and respect `reduceMotion()`. Do not increase resource usage with permanent compositing hints or multiple large blurred layers.

### Playback, Protocol, and Data

- Successful room joins authenticate first, close the controlled dialog, and enter the listening view only after the dialog reports it has closed. Keep the Dialog mounted while its `open` becomes false so dismissal can complete. Closing a pending join cancels the attempt; stale completions cannot navigate.
- A new song or a new playback start resets lyric scrolling immediately, cancels pending centering and highlight motion, and aligns the new lyrics without carrying over the old song's progress. Reconnecting to the same playback keeps its state; normal line changes retain smooth following. Ignore progress callbacks from replaced or paused audio sources.
- Key native lyric rows by index and text so changed lines remount and report measurements even when their height is unchanged. Centering needs measured rows only through the target (or the next visible line during a blank timestamp), without waiting for the rest of the song. Reuse valid measurements of unchanged text.

- On mobile, the All search results preview at most four playlists in a fixed two-column grid (two rows when four results are available). Show All opens the full playlist results; desktop previews remain unchanged.

- Search sections publish as soon as their own HTTP request finishes; a slow playlist request must not hide completed song results (or vice versa). Artwork reserves its space immediately and displays a local placeholder until that individual image loads. Each cover replaces its placeholder independently; missing or failed images retain the placeholder without blocking text, controls, or other covers. Do not preload a whole search page behind a shared image-ready barrier.

- Run NSH `Video.onLoad` initialization only once per source load, preventing seeks from triggering another synchronization and causing stutter.
- Repeated playback snapshots must not reload, seek, or restart playback. Explicit synchronization may recalibrate the same song. Wait for future start times; retain metadata for completed songs while keeping audio paused.
- UI song durations use milliseconds, converting to seconds only for display or audio APIs. Preserve server clock offset and start-time synchronization.
- Preserve unlimited WebSocket reconnection with exponential backoff, heartbeat clock calibration, and recovery on returning to the foreground or regaining the network. Do not repeatedly reconnect because heartbeat responses are absent.
- On server disconnect, connection error, or message write failure, clean up the invalid connection and retry automatically after 3 / 6 / 12 / 24 / 30 seconds, capped at 30 seconds with unlimited attempts. Delayed events from an old connection must not overwrite the new connection's state. After recovery, refetch playback, queue, members, and settings. A user-initiated disconnect stops reconnection.
- Requests and events use only the livehouse v2 protocol. Apply revisioned snapshots and never automatically replay writes after disconnection.
- Accept search results only from the latest request. Support cancellation for HTTP queries to prevent stale results from replacing the current view.
- Rooms, songs, users, and online status come from real data. Do not fill the production interface with demo content.

## 10. Implementation Entry Points and Acceptance


| Area | Entry points |
| --- | --- |
| Themes, font sizes, layout, and component styles | `src/theme.ts`, `src/stores/appearance-store.ts` |
| Top navigation, page switching, and right panel | `src/app.tsx` |
| Search entry and results | `src/components/header-search.tsx`, `src/components/search-page.tsx` |
| Room discovery, joining, and room creation | `src/components/room-browser.tsx` |
| Lyrics-only home page and synchronization | `src/components/listen-page.tsx`, `src/components/lyrics.tsx`, `src/stores/lyrics-store.ts` |
| Bottom player and cross-platform audio | `src/components/player-bar.tsx`, `src/components/audio-engine.tsx` |
| Shared controls, dialogs, and notifications | `src/components/ui.tsx`, `src/components/notifications.tsx` |
| Performance levels | `src/stores/performance-store.ts`, `src/theme.ts` |
| Web-only integration and storage migration | `host.js`, `src/utils/runtime.ts`, `src/utils/storage.ts` |

Manually check UI changes in a real browser across dark and light themes, desktop and narrow screens, right-panel toggles, long text, font-size settings, keyboard operation, and reduced motion. Follow the testing rules in `AGENTS.md`: do not write UI or frontend unit tests, especially interface numeric or layout assertions. Changes to logic, types, or styles require at least `npm run type-check` and `npm run lint`. Changes affecting the build or dependencies require `npm run build`. Manually verify the relevant flows when changing playback, protocol behavior, or related interactions.

Last updated: October 18, 2026.
