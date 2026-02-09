# Sphere Skills

This directory contains AI skills for the Sphere ecosystem. The primary skill is `sphere-framework`, which helps an AI agent choose the right Sphere component, follow the correct code-generation workflow, and avoid editing generated files.

## What This Skill Is For

Use `sphere-framework` when working on:

- Sphere Protocol-First backend development
- Protobuf + codegen workflow (`protoc-gen-sphere*`, `protoc-gen-route`)
- Sphere runtime packages (`cache`, `mq`, `storage`, `server`, `core`, `utils`, `infra`)
- Sphere templates (`sphere-layout`, `sphere-simple-layout`, `sphere-bun-layout`)
- Supporting libraries (`httpx`, `confstore`, `entc-extensions`)

## What You Get

- A decision-oriented `SKILL.md` for day-to-day execution
- Focused reference docs for API, ORM, auth, troubleshooting, and package selection
- Clear source-of-truth boundaries (what to edit vs what is generated)
- Standard generation order aligned with Sphere layout best practices

## Installation

### Option 1: Manual Install (recommended if local)

Copy the skill folder into your Codex skills directory:

```bash
mkdir -p "$CODEX_HOME/skills"
cp -R skills/sphere-framework "$CODEX_HOME/skills/sphere-framework"
```

### Option 2: Install with Skill Installer

If you use the built-in `skill-installer` workflow, install this skill from the repository path instead of copying files manually.

## Activation

The skill is triggered when the task clearly matches Sphere framework work, or when explicitly referenced (for example: `use sphere-framework`).

## Skill Structure

- `skills/sphere-framework/SKILL.md`: execution guide and constraints
- `skills/sphere-framework/references/`: deep-dive docs loaded only when needed
- `skills/sphere-framework/assets/`: optional output assets (minimal by default)
- `skills/sphere-framework/scripts/`: optional automation scripts (minimal by default)

## Version

- Version: 2.1.0
- Scope: Sphere monorepo
- Last updated: 2026-02-09
