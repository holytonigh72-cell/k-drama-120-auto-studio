---
name: k-drama-120-auto-studio
description: Plan and orchestrate an original 120-minute Korean photorealistic romance drama production, using available Avocado AI integrations when verified, with strict cost approval, continuity, audio, editing and upload gates.
---

# K-Drama 120 Auto Studio — Codex Skill

## Mission
Turn one Korean-language premise into a production-ready **original** Korean live-action-style romance drama with a target finished runtime of **120 minutes (7,200 seconds)**. Orchestrate creation only through actually available, authorized tools. Never claim that a script or storyboard is a finished video. Default to **PLAN ONLY**.

## Mandatory production rules
1. **Photorealistic live action only.** Reject cartoon, webtoon, comic, anime, illustration, 3D-render or game-like visuals. Natural Korean actors, realistic skin, anatomy, subtle emotions, plausible cinematography.
2. **Original IP only.** Do not copy copyrighted scripts, distinctive characters, shots, music, or scenes from existing Chinese short dramas or Korean dramas. Genre conventions and general pacing are fine.
3. **Approval gates.** Even if the user says '제작 시작', first show available models, estimated credits/KRW, runtime feasibility, planned outputs, and ask for explicit authorization before any chargeable generation, external upload, or public publication. Never silently spend credits.
4. **Do not generate unused audio.** Only produce approved dialogue/voice assets referenced in the final edit decision list. Turn OFF model-generated audio, speech, SFX and music by default when the API exposes those controls; if not possible, disclose the limitation and do not assume it is disabled. Create Korean dialogue, SFX, and licensed/original music separately only when needed.
5. **Model and integration verification.** Check Codex-accessible tools, Avocado AI MCP availability, callable methods, model names, resolutions, costs, output formats, queue/status, download links, and license. A Claude MCP connection does NOT imply Codex access. Never fabricate endpoints or prices. If unavailable, output ready-to-use prompts/manifests and integration steps instead.
6. **Character continuity.** Maintain character bible with face reference IDs, ages (all adult), wardrobe IDs, hair, locations, props, voice IDs, and timeline. Use approved reference images and continuity checks; flag face drift and rework only approved shots.
7. **Single shot, single action.** Video prompts specify one clear physical action, restrained expressions, consistent clothes/face, camera movement (prefer locked), scene lighting, shot length, and negative constraints. Keep dialogue timing realistic and lip-sync verified rather than assuming generation handles Korean speech.
8. **Budget-aware.** Count all video generations, retries, image generations, TTS, storage, processing and API charges. Distinguish subscription fees from API credits. Enforce user-set caps; default test cap KRW 10,000 **only for the pilot**, not the 120-minute final film. Stop if estimated or observed cost exceeds approved budget.
9. **Upload safety.** Prepare title, description, thumbnail, chapters, captions and disclosure notes. YouTube API uploads require valid credentials and user approval. Default visibility PRIVATE. Obtain separate approval to publish publicly. Comply with synthetic/altered content disclosure requirements when applicable.
10. **No fictional completion.** Report exact artifact paths, generated durations, failed jobs, and unfilled scenes. Do not say 120 minutes completed unless rendered file runtime is verified.

## Workflow
### Stage 0 — Discover and plan (no generation)
- Confirm premise, genre, intended audience, episode/film format, visual reference, available tools, target budget, and target output (16:9 1920x1080 by default, 24 fps).
- Inventory Avocado MCP in **this** Codex environment; if missing, propose verified alternative/manual bridge, not an invented integration.
- Show 120-minute feasibility: 7,200 seconds / typical 5-second shots = 1,440 clips if every second were generative video; plan selective motion, held compositions, dialogue-driven scenes, montage, and economical reuse only where narratively justified. Avoid promising cheap full-motion 120-minute generation.
- Provide a pilot budget and full-production estimate with assumptions, no invented prices.

### Stage 1 — Story bible
Create `project/story_bible.md`: logline, original premise, hook, theme, protagonist goals, stakes, 3-act/8-sequence structure, 12–20 meaningful turning points, 2–4 main adult characters, supporting cast, 120-minute time-coded beat sheet. Maintain causality, romantic development, and cliffhangers.

### Stage 2 — Screenplay and breakdown
Create `project/screenplay.md` with numbered scenes, timecodes, dialogue, actions, sound notes and transitions. Create `project/scenes.csv` with scene_id, act, start_s, duration_s, location, cast, wardrobe_ids, emotion, shot_count, dialogue_ids, status. Sum of planned durations MUST equal 7,200 seconds; verify actual rendered duration separately.

### Stage 3 — Character and style assets
Create `project/character_bible.md`, `project/style_bible.md`, and `project/shot_manifest.csv`. Set stable image references and per-shot prompts. Prompt in English for visuals if model benefits, Korean for spoken dialogue. Never use real actors' likenesses without permission.

### Stage 4 — Pilot validation (explicit approval)
Produce a 60–90 second pilot only after approval. Validate face consistency, motion quality, natural Korean speech, audio sync, subtitles, render pipeline, and per-minute cost. Present findings before scaling.

### Stage 5 — Batch production (explicit approval)
Produce in independently resumable batches (e.g. 5–10 minute sequences). Track generation IDs, model, seed/reference, attempts, unit cost, status and outputs in `project/jobs.csv`. Never regenerate successful clips without reason. Require confirmation for new budget tranches. Fail gracefully and leave resumable state.

### Stage 6 — Post-production
Use FFmpeg or other verified local tools to assemble approved clips, dialogue, ambience, music, and captions. Check audio loudness, transitions, subtitle timing, black frames, freeze errors, repeated shots, face drift, and full 7,200-second target. Deliver MP4 plus SRT, thumbnail and metadata. If exact runtime cannot be achieved without padding, report it and request a story/edit decision.

### Stage 7 — Upload (explicit approval)
Show final video, runtime, title, description, disclosures and visibility. Upload privately only after approval; request separate public-release approval.

## Invocation and response format
When user says `주제: ...`, output: (A) concept + hook, (B) 120-minute structure, (C) cast bible, (D) estimated shot count and cost assumptions, (E) tool integration verification results, (F) next approval request. **Do not generate media.**
When user says `파일럿 제작 승인`, execute only the specifically approved pilot and spending cap if tools are available.
When user says `본편 제작 승인`, first confirm scope and approved batch budget; never interpret it as unlimited spending.

## Quality checklist
Read `references/quality-checklist.md` and enforce every applicable item before final render or upload.
