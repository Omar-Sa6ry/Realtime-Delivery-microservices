import * as path from 'path';
import { Global, Module } from '@nestjs/common';
import {
  AcceptLanguageResolver,
  HeaderResolver,
  I18nModule,
} from 'nestjs-i18n';

@Global()
@Module({
  imports: [
    I18nModule.forRoot({
      fallbackLanguage: 'en',
      loaderOptions: {
        path: path.join(process.cwd(), 'src/common/translation/locales/'),
        watch: true,
      },
      resolvers: [new HeaderResolver(['x-lang']), new AcceptLanguageResolver()],
    }),
  ],
  exports: [I18nModule],
})
export class TranslationModule {}
