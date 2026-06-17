import en from './locales/en.json' with { type: 'json' };

export type Locale = 'en' | 'zh-CN';

export const supportedLocales = ['en', 'zh-CN'] as const satisfies readonly Locale[];

export type TranslationTree = typeof en;

type DotPrefix<TPrefix extends string, TKey extends string> = TPrefix extends ''
  ? TKey
  : `${TPrefix}.${TKey}`;

export type TranslationKey<T = TranslationTree, TPrefix extends string = ''> = {
  [K in keyof T & string]: T[K] extends string
    ? DotPrefix<TPrefix, K>
    : TranslationKey<T[K], DotPrefix<TPrefix, K>>;
}[keyof T & string];
