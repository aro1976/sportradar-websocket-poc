#!/usr/bin/env node
import * as cdk from 'aws-cdk-lib/core';
import { InfraStack } from '../lib/infra-stack';

const app = new cdk.App();
new InfraStack(app, 'WebSocketPocStack', {
  env: { account: process.env.CDK_DEFAULT_ACCOUNT, region: 'sa-east-1' },
});
