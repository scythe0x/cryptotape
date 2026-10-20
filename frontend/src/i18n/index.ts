import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import { en } from './en'
import { ru } from './ru'

export { useTranslation } from 'react-i18next'
export type TranslationKey = keyof typeof en

void i18n.use(initReactI18next).init({
  resources: { en: { translation: en }, ru: { translation: ru } },
  lng: 'ru',
  fallbackLng: 'ru',
  supportedLngs: ['en', 'ru'],
  initAsync: false,
  interpolation: { escapeValue: false },
  react: { useSuspense: false },
})

export function applyLanguage(language: 'en' | 'ru') {
  if (i18n.language !== language) void i18n.changeLanguage(language)
  if (typeof document !== 'undefined') document.documentElement.lang = language
}

export function soundLabel(id: string): string {
  if (id.startsWith('custom:')) return id.slice(7).replace(/^\d+-/, '')
  switch (id) {
    case 'none':
      return i18n.t('noSound')
    case 'bell':
    case 'chime':
    case 'notification':
    case 'alarm':
    case 'beep':
      return i18n.t(id)
    default:
      return id
  }
}

export default i18n
