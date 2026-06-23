# Changelog

All notable changes to Koffy are documented in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Independently uploadable and persistent user-center and admin favicons.
- Full production Compose stack for a blank Docker host.

### Changed

- Production Nginx and LiteLLM custom configuration now uses ignored runtime files copied from tracked examples.

## [0.1.0] - 2026-06-18

### Added

- Self-hosted user center and administration console.
- Casdoor-backed authentication with phone/password and optional WeChat login.
- Wallets, points ledger, plans, subscriptions, and monthly entitlements.
- Recharge orders and optional WeChat Pay integration.
- OpenAI-compatible Koffy Gateway with application keys, rate limits, LiteLLM routing, usage pre-authorization, and settlement.
- Optional SMS verification and Tencent CAPTCHA integration.
- Docker Compose examples for local development and production deployment.
