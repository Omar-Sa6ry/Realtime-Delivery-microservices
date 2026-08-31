FROM node:24.11.0

WORKDIR /usr/src/app

COPY packages/ts ./packages/ts
WORKDIR /usr/src/app/packages/ts
RUN npm install --legacy-peer-deps
RUN npm run build

WORKDIR /usr/src/app
COPY services/delivery-service/package.json ./services/delivery-service/

WORKDIR /usr/src/app/services/delivery-service
RUN npm install --legacy-peer-deps

# Clean up duplicate modules from packages/ts to resolve NestJS DI token duplication
RUN rm -rf /usr/src/app/packages/ts/node_modules/nestjs-i18n \
           /usr/src/app/packages/ts/node_modules/@nestjs \
           /usr/src/app/packages/ts/node_modules/@grpc \
           /usr/src/app/packages/ts/node_modules/express \
           /usr/src/app/packages/ts/node_modules/graphql \
           /usr/src/app/packages/ts/node_modules/@apollo

# Symlink the delivery-service node_modules up so packages/ts resolves them to the same instance
RUN ln -s /usr/src/app/services/delivery-service/node_modules /usr/src/app/node_modules

WORKDIR /usr/src/app
COPY . .

WORKDIR /usr/src/app/services/delivery-service
RUN npm run build

CMD ["npm", "run", "start:prod"]

