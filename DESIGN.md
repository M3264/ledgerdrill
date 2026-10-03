---
name: LedgerDrill
description: Controlled faults and readable payment evidence
colors:
  paper: '#f3f1e9'
  ink: '#222922'
  muted: '#5d665b'
  line: '#cfcec2'
  fault: '#a34122'
  verified: '#286141'
  selected: '#fffdf7'
  panel: '#e8e8df'
typography:
  display:
    fontFamily: 'Space Grotesk Variable, sans-serif'
    fontSize: 'clamp(36px, 4vw, 58px)'
    fontWeight: 500
    lineHeight: 1.06
    letterSpacing: '-0.035em'
  title:
    fontFamily: 'Space Grotesk Variable, sans-serif'
    fontSize: '22px'
    fontWeight: 500
    letterSpacing: '-0.025em'
  evidence:
    fontFamily: 'IBM Plex Mono, monospace'
    fontSize: '9px'
rounded:
  button: '7px'
  scenario: '8px'
spacing:
  compact: '12px'
  section: '24px'
  wide: '32px'
components:
  button-primary:
    backgroundColor: '{colors.fault}'
    textColor: '{colors.selected}'
    rounded: '{rounded.button}'
    padding: '12px 16px'
    height: '46px'
---

## Overview

LedgerDrill is a diagnostic instrument on warm paper. Flat sections, precise rules and restrained color keep the payment evidence readable. The recurring signature is the paired client comparison: an unsafe retry beside a client that retains its original transaction.

## Colors

**The Evidence Rule.** Burnt orange identifies the action that introduces a fault and the resulting uncertainty or duplicate payment. Green accompanies observed confirmation. Status words and transaction counts always carry the meaning too.

Neutral paper spans the full canvas. A pale panel groups selectable scenarios and the empty-state flow. Selected controls use a lighter surface and an ink outline.

## Typography

Space Grotesk carries the interface, with medium-weight headlines and plain explanatory text. IBM Plex Mono carries hashes, times, network labels and measured amounts. Fonts are self-hosted. Numeric comparisons use tabular figures.

## Layout

A full-width navigation bar leads to a narrow control column and a broad evidence area. At tablet widths, the two clients stack. At mobile widths, controls and results become a single reading sequence; the header wraps and history retains a horizontally scrollable table. Breakpoints are 1050px and 700px; wide desktop spacing changes at 1500px.

## Elevation & Depth

The interface is flat. Thin rules divide work areas and timeline entries. Tonal surfaces group content without shadows or floating overlays.

## Shapes

Rounded controls are gently softened; navigation and content remain open. Scenario controls keep native radio inputs inside one clickable label. Thin rules give tables and evidence their structure.

## Components

Primary buttons name the immediate operation and display its active state. The three fault options include both a title and a plain-language consequence. Result columns present intended behavior, measured settlement counts, a verdict, a timeline and real transaction links. Empty states explain the path before a run exists; they contain no invented measurements.

Focus uses a visible 3px outline. Errors use an alert region. Run status is announced through a status region. Motion is limited to a short entrance of the status label and color transitions; reduced-motion preferences disable both.

## Do's and Don'ts

Use short explanatory sentences near the relevant evidence. Keep full hashes in downloadable reports and readable shortened links in the interface. Preserve native links, radio inputs, selects and keyboard focus.

Do not imply an app-wide certification from one scenario. Do not show synthetic measurements as live results. Do not make color the only distinction between a recovered payment and a duplicate.
