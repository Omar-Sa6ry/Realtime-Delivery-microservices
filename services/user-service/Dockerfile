FROM node:24.11.0

WORKDIR /usr/src/app

COPY packages/ts ./packages/ts
WORKDIR /usr/src/app/packages/ts
RUN npm install
RUN npm run build

WORKDIR /usr/src/app
COPY services/user-service/package.json ./services/user-service/

WORKDIR /usr/src/app/services/user-service
RUN npm install --legacy-peer-deps

# Clean up duplicate modules from packages/ts to resolve NestJS DI token duplication
RUN rm -rf /usr/src/app/packages/ts/node_modules/nestjs-i18n \
           /usr/src/app/packages/ts/node_modules/@nestjs \
           /usr/src/app/packages/ts/node_modules/@grpc \
           /usr/src/app/packages/ts/node_modules/express

# Symlink the user-service node_modules up so packages/ts resolves them to the same instance
RUN ln -s /usr/src/app/services/user-service/node_modules /usr/src/app/node_modules

WORKDIR /usr/src/app
COPY . .

WORKDIR /usr/src/app/services/user-service
CMD ["npm", "run", "start:dev"]
