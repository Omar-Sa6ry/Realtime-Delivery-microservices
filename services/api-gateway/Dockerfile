FROM node:24.11.0

WORKDIR /usr/src/app

COPY packages/ts ./packages/ts
WORKDIR /usr/src/app/packages/ts
RUN npm install
RUN npm run build

WORKDIR /usr/src/app
COPY services/api-gateway/package.json ./services/api-gateway/

WORKDIR /usr/src/app/services/api-gateway
RUN npm install --legacy-peer-deps

WORKDIR /usr/src/app
COPY . .

WORKDIR /usr/src/app/services/api-gateway
CMD ["npm", "run", "start:dev"]
