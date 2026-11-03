import { DEFAULT_LOCALE, type Locale, resolveLocale } from "@/i18n/config";

import packageJson from "../../package.json";

const currentYear = new Date().getFullYear();

// 브라우저 탭 제목·검색엔진 메타 설명. layout.tsx 의 정적 metadata 로 들어가 모든
// 페이지의 <title>·<meta name="description"> 에 렌더되므로, 빌드 기본 locale 에 맞춰
// 다섯 언어를 분기한다. zh 는 상류 대조를 위해 원문을 그대로 보존한다. 제목은 엠대시 대신
// 콜론을 써서 "제품명: 설명" 형태로 둔다. 이 메타는 빌드 시점 정적 값이라 실행 중 언어
// 전환에는 반응하지 않는다(탭 제목 한 줄 수준이라 허용한다).
const META_BY_LOCALE: Record<Locale, { title: string; description: string }> = {
  en: {
    title: "TRACE: Threat Reconstruction & Analysis Coordination Engine",
    description: "Agentic digital forensics and threat-hunting platform",
  },
  ko: {
    title: "TRACE: 위협 재구성 및 분석 조정 엔진",
    description: "에이전트 기반 디지털 포렌식 및 위협 헌팅 플랫폼",
  },
  zh: {
    title: "TRACE — 威胁重建与分析协调引擎",
    description: "智能体驱动的数字取证与威胁狩猎平台",
  },
  es: {
    title: "TRACE: Motor de Reconstrucción de Amenazas y Coordinación de Análisis",
    description: "Plataforma de análisis forense digital y búsqueda de amenazas con agentes de IA",
  },
  ja: {
    title: "TRACE: 脅威再構築・分析調整エンジン",
    description: "エージェント型デジタルフォレンジックおよび脅威ハンティングプラットフォーム",
  },
};

export const APP_CONFIG = {
  name: "TRACE",
  version: packageJson.version,
  copyright: `© ${currentYear}, TRACE.`,
  meta: META_BY_LOCALE[resolveLocale()] ?? META_BY_LOCALE[DEFAULT_LOCALE],
};
