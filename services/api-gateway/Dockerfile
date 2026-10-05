FROM node:24.11.0

WORKDIR /usr/src/app
COPY services/api-gateway/package*.json ./services/api-gateway/

WORKDIR /usr/src/app/services/api-gateway
RUN npm install --legacy-peer-deps
RUN npm install graphql@16.10.0 --save-exact --legacy-peer-deps


WORKDIR /usr/src/app
COPY . .

WORKDIR /usr/src/app/services/api-gateway
RUN npm run build

CMD ["node", "dist/main"]
