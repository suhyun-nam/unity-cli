# Changelog

fork(`suhyun-nam/unity-cli`)의 릴리스 기록이다. 원본은 [youngwoocho02/unity-cli](https://github.com/youngwoocho02/unity-cli)이고, 여기에는 fork에서 바뀐 것만 적는다.

- 형식은 [Keep a Changelog](https://keepachangelog.com/ko/1.1.0/)를 따른다. 분류는 `Added`(추가), `Changed`(변경), `Fixed`(버그 수정).
- 항목마다 첫 줄에 기반 upstream을 적는다. upstream `main`이 마지막 태그보다 앞서 있으면 `v0.4.1 + 07c62fd`처럼 태그와 커밋을 함께 적는다.
- 릴리스 전 변경은 `[Unreleased]`에 쌓고, 릴리스할 때 그 절을 버전 제목으로 바꾼다.
- 0.4.2 이전 버전은 [upstream 릴리스 노트](https://github.com/youngwoocho02/unity-cli/releases)를 본다.

## [Unreleased]

기반 upstream: `v0.4.1 + 07c62fd`

### Added
- fork 릴리스 기록 `CHANGELOG.md`를 둔다.

## [0.4.2] - 2026-11-28

기반 upstream: `v0.4.1 + 07c62fd`

### Fixed
- PlayMode 테스트의 대기·결과 파일을 프로젝트별 status 폴더로 나눈다. 에디터 여러 개가 같은 폴더를 쓰면 다른 프로젝트의 결과를 자기 결과로 받던 문제. CLI는 커넥터가 알려 준 이번 실행의 결과 파일만 기다린다. ([#2](https://github.com/suhyun-nam/unity-cli/pull/2))
- 커넥터가 Asset Import Worker 프로세스에서 시작하지 않는다. worker가 같은 프로젝트의 인스턴스 파일을 덮어쓰고 종료 때 stopped를 남기던 문제. ([#2](https://github.com/suhyun-nam/unity-cli/pull/2), upstream [#37](https://github.com/youngwoocho02/unity-cli/issues/37))
- `exec`는 코드를 인자나 `--code`로 주면 stdin을 읽지 않는다. stdin이 닫히지 않는 파이프(에이전트 셸, CI, ssh)에서 응답 없이 멈추던 문제. help와 README에 동작을 적었다. ([#3](https://github.com/suhyun-nam/unity-cli/pull/3), upstream [#38](https://github.com/youngwoocho02/unity-cli/issues/38))

### Changed
- 저장소 주소를 fork로 바꾼다: Go 모듈 경로, `unity-cli update`가 조회하는 릴리스, 설치 스크립트, README의 설치 URL. ([#1](https://github.com/suhyun-nam/unity-cli/pull/1))
