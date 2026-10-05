FROM node:24.11.0

WORKDIR /usr/src/app
COPY services/delivery-service/package*.json ./services/delivery-service/

WORKDIR /usr/src/app/services/delivery-service
RUN npm install --legacy-peer-deps
RUN npm install graphql@16.10.0 --save-exact --legacy-peer-deps


WORKDIR /usr/src/app
COPY . .

WORKDIR /usr/src/app/services/delivery-service
RUN npm run build

CMD ["npm", "run", "start:prod"]

