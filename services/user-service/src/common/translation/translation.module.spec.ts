import { GLOBAL_MODULE_METADATA } from '@nestjs/common/constants';
import { TranslationModule } from './translation.module';

describe('TranslationModule', () => {
  const metadata = (key: string): any[] =>
    (Reflect.getMetadata(key, TranslationModule) as any[]) ?? [];

  it('is registered as a global module', () => {
    expect(Reflect.getMetadata(GLOBAL_MODULE_METADATA, TranslationModule)).toBe(
      true,
    );
  });

  it('imports the i18n root module', () => {
    const i18n = metadata('imports').find(
      (m) => m?.module?.name === 'I18nModule',
    );

    expect(i18n).toBeDefined();
  });

  it('defaults to English and reads the locale JSON files from src', () => {
    const i18n = metadata('imports').find(
      (m) => m?.module?.name === 'I18nModule',
    );
    const options = i18n.providers.find(
      (p: any) => p.useValue?.fallbackLanguage !== undefined,
    ).useValue;

    expect(options.fallbackLanguage).toBe('en');
    expect(options.loaderOptions.path.replace(/\\/g, '/')).toContain(
      'src/common/translation/locales',
    );
    expect(options.loaderOptions.watch).toBe(true);
  });

  it('resolves the language from the x-lang header first', () => {
    const i18n = metadata('imports').find(
      (m) => m?.module?.name === 'I18nModule',
    );
    const options = i18n.providers.find(
      (p: any) => p.useValue?.resolvers !== undefined,
    ).useValue;

    expect(options.resolvers).toHaveLength(2);
    expect(options.resolvers[0].keys).toEqual(['x-lang']);
  });

  it('re-exports the i18n module for consumers', () => {
    const exports = metadata('exports').map((e) => e?.name ?? e);

    expect(exports).toContain('I18nModule');
  });
});
