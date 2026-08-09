FROM node:24.11.0

WORKDIR /usr/src/app

COPY packages/ts ./packages/ts
COPY services/api-gateway/package.json ./services/api-gateway/

WORKDIR /usr/src/app/services/api-gateway
RUN npm install --legacy-peer-deps

WORKDIR /usr/src/app
COPY . .

WORKDIR /usr/src/app/services/api-gateway
CMD ["npm", "run", "start:dev"]
