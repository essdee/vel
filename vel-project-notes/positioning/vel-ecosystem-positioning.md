# Vel Ecosystem Positioning — Final (March 2, 2026)

## The Lineup

| Product | GitHub | One-Liner (GitHub description) |
|---------|--------|-------------------------------|
| **Vel** | essdee/vel | AI-native Go framework. Your agent builds. The framework guarantees. |
| **Velboard** | karthikeyan5/velboard | The dashboard that builds itself. A Vel app. |
| **VelBridge** | karthikeyan5/velbridge | Your agent can use your browser. A Vel app. |

---

## Naming Rules

- **Vel-family, not Claw-family.** "Velboard" ties to Vel (a framework — bigger surface, more longevity). "Clawboard" would tie it to OpenClaw (one platform).
- **"Claw" is overloaded** in the ecosystem (ClawHub, ClawWatcher, OpenClaw). "Vel" is unique and clean.
- **Each name = what you get.** Vel (the spear / the framework). Velboard (a dashboard). VelBridge (a bridge to your browser).
- **Names signal ecosystem.** Seeing "Vel-" tells you immediately it's a Vel app — which IS the selling point.
- **Never say "VelReach" or "VelBrowser"** — deprecated names. Always VelBridge.

---

## Vel (Framework)

### Hero Pitch
> Stop waiting for PRs. Tell your AI agent what you need. It builds it. The framework makes sure it can't break anything else.

### Core Differentiator
The only framework designed so AI agents can safely extend it at runtime. Not "AI-friendly docs" — structural enforcement. Compile-time capability checks, manifest-driven validation, five-function public API.

### Key Lines
- "Your agent writes the app. Vel guarantees it works."
- "The insecure path doesn't exist. The wrong structure is rejected at build time."
- "AI writes two files, and it works. Every time."
- "Don't wait for PRs" — the paradigm shift all three repos share.

### What Vel Is NOT
- NOT "WordPress of the AI world" — WordPress has baggage (bloat, plugins breaking, security). Vel is the opposite: opinionated, enforced, minimal.
- NOT a web framework that happens to work with AI. It's AI-native from the ground up.
- Don't compare to other frameworks. Let Vel be Vel.

---

## Velboard (Dashboard App)

### Hero Pitch
> The dashboard that builds itself. 11 panels on day one — and your agent builds the next ones.

### Core Differentiator
Every other OpenClaw dashboard was built by a human developer who decided what panels you need. Velboard was built by an AI agent, for AI agents, on a framework designed for AI agents. The thing that built it can keep building it. For you. Based on what you actually need.

### Key Lines
- **"The dashboard that builds itself."** — THE tagline. Not "AI-built" as a past-tense badge. AI-built as an ongoing capability.
- **"Don't wait for PRs."** — The main pitch. Stronger than "11 panels." The paradigm shift: need a feature → your agent builds it now → the framework ensures it can't break.
- **"The last dashboard you'll ever install."** — The prompt-to-panel section. "See something you like on someone else's dashboard? Screenshot it, send it to your agent, you have it."
- **"You don't need to know how it works. You just need to know what you want to see. Your agent handles the rest, and the framework makes sure it can't mess it up."** — The trust line.
- "Stop asking. Start seeing." — Supporting line only, not the hero.

### Pitch Hierarchy (README order)
1. "The dashboard that builds itself" (hook)
2. What you get on day one — panel grid (proof it works, not the selling point)
3. "Don't wait for PRs" (paradigm shift)
4. "The last dashboard you'll ever install" (screenshot-to-panel prompt tip)
5. Cross-reference to VelBridge
6. Built on Vel

### What Velboard Is NOT
- NOT a "panel pack" or "monitoring plugin" — that undersells it
- NOT positioned on feature count ("11 panels!") — positioned on extensibility
- Don't lead with technical details (manifest structure, WebSocket internals)
- Don't show code in the pitch sections — zero tech barrier in the selling part

---

## VelBridge (Browser Relay App)

### Hero Pitch
> Your agent can use YOUR browser. No passwords shared. You're already logged in.

### Core Differentiator
Your agent doesn't need your credentials. It uses the browser where you're already authenticated. Pair with a code, watch it work, unpair anytime. Full transparency, full control.

### Key Lines
- **"Your agent can use YOUR browser."** — "your" is the critical word. Not A browser. YOUR browser. The one with your cookies, sessions, saved passwords.
- **"No passwords shared."** — You're already logged in. Your agent just uses that session.
- **"Watch your agent work in real time."** — Transparency builds trust. It's your Chrome, on your machine.
- **"Human-like interaction."** — Bezier curve mouse movements, not teleporting cursors. Sites can't tell.
- **"You stay in control."** — Pair with a 6-character code. Unpair anytime. Your machine, your rules.

### Pitch Hierarchy (README order)
1. "Your agent can use your browser" (hook)
2. Why this matters — no passwords, already logged in (trust)
3. How it works — pair, watch, unpair (simplicity)
4. What your agent can do (capabilities without endpoint tables)
5. "Don't wait for PRs" (shared ecosystem message)
6. Cross-reference to Velboard
7. Built on Vel

### What VelBridge Is NOT
- NOT a "WebSocket relay" or "CDP proxy" — that's plumbing, not positioning
- NOT a standalone browser — it bridges agent ↔ your existing browser
- Don't lead with architecture diagrams or endpoint tables
- Don't expose internal component names (SessionManager, PairingManager) in the README
- Technical docs belong in separate files (ARCHITECTURE.md, API.md)

---

## Shared Messaging

### "Don't Wait for PRs" (appears in ALL three repos)
The pitch adapts per repo but the core message is identical:
- **Vel:** "Every framework has this bottleneck: you need a feature, you open an issue, maybe someone builds it. Vel is different."
- **Velboard:** "Need a panel that doesn't exist? Your agent builds it. Right now."
- **VelBridge:** "Need a browser capability that doesn't exist? Your agent extends it."

### README Structure (all repos follow this)
1. Hook / one-liner
2. What you get (proof it works)
3. "Don't wait for PRs" (paradigm shift)
4. The "last X you'll ever install" angle
5. Cross-references to other Vel apps
6. Built on Vel (for apps) / Apps table (for framework)

### Tone
- Confident, not salesy
- Direct, not breathless
- Show don't tell — then tell anyway because it's that good
- Zero filler words. Zero "powerful" / "seamlessly" / "cutting-edge"
- Tech details in separate docs, not the README pitch

---

## Competitive Context (intel, not for READMEs)

### OpenClaw Dashboard Landscape (as of Feb 2026)
1. **tugcantopaloglu/openclaw-dashboard** — session management, cost tracking, live feed, memory browser, TOTP MFA. Node.js.
2. **mudrii/openclaw-dashboard** — 11 panels, cost donuts, cron status, sub-agent tracking. "Command center." Zero deps.
3. **abhi1693/openclaw-mission-control** — team/org orchestration, governance, approval flows. Enterprise.
4. **ClawWatcher** — token/cost tracking, action visibility.

All fight on the same ground: "see your agent's status without asking." Feature list vs feature list.

**Velboard wins because it's extensible by your agent.** Not because it has more panels. The panels are proof. The extensibility is the product.

### Why Nobody Else Has This
They're all static dashboards. Fixed features, fixed panels. Want something new? File an issue, wait for a PR, maybe fork. Velboard is the only one where the agent that uses it can also extend it — because Vel (the framework) was designed for exactly this.

---

## Decision Log

| Date | Decision | Context |
|------|----------|---------|
| 2026-02-28 | "Dashboard that builds itself" = Velboard tagline | Karthi + Ram positioning session |
| 2026-02-28 | "Don't wait for PRs" = primary pitch for all repos | Stronger than feature count |
| 2026-02-28 | Browser relay → separate repo/app | Seeing ≠ Doing, different products |
| 2026-02-28 | Vel-family naming over Claw-family | Framework > platform tie-in |
| 2026-02-28 | "Your agent can use YOUR browser" | "your" = trust + convenience |
| 2026-02-28 | Drop "WordPress of the AI world" comparison | Baggage: bloat, insecurity |
| 2026-03-02 | VelBrowser → VelBridge (final name) | "Bridge" = accurate, not a browser |
| 2026-03-02 | Velboard + VelBridge fully independent | Only shared dep is Vel framework |
| 2026-03-02 | App landing page override | First app with landingPage in app.json wins |
