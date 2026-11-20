import { AppController } from './app.controller';

describe('AppController', () => {
  let controller: AppController;

  beforeEach(() => {
    controller = new AppController();
  });

  describe('pingForApiGateway', () => {
    it('returns a successful boolean envelope', () => {
      expect(controller.pingForApiGateway()).toEqual({
        success: true,
        statusCode: 200,
        message: 'Api Gateway is running',
        data: true,
      });
    });

    it('reports data as true (gateway is alive)', () => {
      expect(controller.pingForApiGateway().data).toBe(true);
    });

    it('uses HTTP 200 as the status code', () => {
      expect(controller.pingForApiGateway().statusCode).toBe(200);
    });
  });
});
