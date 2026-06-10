import * as cdk from 'aws-cdk-lib/core';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as ecs from 'aws-cdk-lib/aws-ecs';
import * as elbv2 from 'aws-cdk-lib/aws-elasticloadbalancingv2';
import * as elasticache from 'aws-cdk-lib/aws-elasticache';
import * as logs from 'aws-cdk-lib/aws-logs';
import * as ecr_assets from 'aws-cdk-lib/aws-ecr-assets';
import * as path from 'path';
import { Construct } from 'constructs';

const VPC_ID = 'vpc-02e80ab39afd3222e';
const SUBNET_IDS = ['subnet-03d9801b371ce4edb', 'subnet-0e8918f335a5b279b'];
const AZS = ['sa-east-1a', 'sa-east-1b'];

export class InfraStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    // Import existing VPC and subnets
    const vpc = ec2.Vpc.fromVpcAttributes(this, 'Vpc', {
      vpcId: VPC_ID,
      availabilityZones: AZS,
      publicSubnetIds: SUBNET_IDS,
    });

    const subnetSelection: ec2.SubnetSelection = {
      subnets: SUBNET_IDS.map((id, i) =>
        ec2.Subnet.fromSubnetAttributes(this, `Subnet${i}`, {
          subnetId: id,
          availabilityZone: AZS[i],
        })
      ),
    };

    // Security Groups
    const redisSg = new ec2.SecurityGroup(this, 'RedisSG', { vpc });
    const ecsSg = new ec2.SecurityGroup(this, 'EcsSG', { vpc });

    redisSg.addIngressRule(ecsSg, ec2.Port.tcp(6379), 'ECS to Redis');
    ecsSg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(8080), 'NLB to ECS');

    // ElastiCache Redis
    const redisSubnetGroup = new elasticache.CfnSubnetGroup(this, 'RedisSubnets', {
      description: 'Redis subnet group',
      subnetIds: SUBNET_IDS,
    });

    const redis = new elasticache.CfnCacheCluster(this, 'Redis', {
      engine: 'redis',
      cacheNodeType: 'cache.t4g.micro',
      numCacheNodes: 1,
      vpcSecurityGroupIds: [redisSg.securityGroupId],
      cacheSubnetGroupName: redisSubnetGroup.ref,
    });

    const redisAddr = `${redis.attrRedisEndpointAddress}:${redis.attrRedisEndpointPort}`;

    // ECS Cluster
    const cluster = new ecs.Cluster(this, 'Cluster', { vpc });

    // Docker images
    const projectRoot = path.join(__dirname, '../../');

    const ingestImage = new ecr_assets.DockerImageAsset(this, 'IngestImage', {
      directory: projectRoot,
      buildArgs: { SERVICE: 'ingest' },
      platform: ecr_assets.Platform.LINUX_AMD64,
    });

    const wsImage = new ecr_assets.DockerImageAsset(this, 'WsImage', {
      directory: projectRoot,
      buildArgs: { SERVICE: 'wsserver' },
      platform: ecr_assets.Platform.LINUX_AMD64,
    });

    // Ingest Service
    const ingestTaskDef = new ecs.FargateTaskDefinition(this, 'IngestTask', {
      cpu: 256,
      memoryLimitMiB: 512,
    });

    ingestTaskDef.addContainer('ingest', {
      image: ecs.ContainerImage.fromDockerImageAsset(ingestImage),
      environment: { REDIS_URL: redisAddr },
      logging: ecs.LogDrivers.awsLogs({
        streamPrefix: 'ingest',
        logRetention: logs.RetentionDays.THREE_DAYS,
      }),
    });

    new ecs.FargateService(this, 'IngestService', {
      cluster,
      taskDefinition: ingestTaskDef,
      desiredCount: 1,
      securityGroups: [ecsSg],
      assignPublicIp: true,
      vpcSubnets: subnetSelection,
    });

    // WebSocket Service
    const wsTaskDef = new ecs.FargateTaskDefinition(this, 'WsTask', {
      cpu: 1024,
      memoryLimitMiB: 2048,
    });

    wsTaskDef.addContainer('wsserver', {
      image: ecs.ContainerImage.fromDockerImageAsset(wsImage),
      environment: { REDIS_URL: redisAddr, WS_ADDR: ':8080' },
      portMappings: [{ containerPort: 8080 }],
      logging: ecs.LogDrivers.awsLogs({
        streamPrefix: 'wsserver',
        logRetention: logs.RetentionDays.THREE_DAYS,
      }),
    });

    const wsService = new ecs.FargateService(this, 'WsService', {
      cluster,
      taskDefinition: wsTaskDef,
      desiredCount: 3,
      securityGroups: [ecsSg],
      assignPublicIp: true,
      vpcSubnets: subnetSelection,
    });

    // NLB
    const nlb = new elbv2.NetworkLoadBalancer(this, 'NLB', {
      vpc,
      internetFacing: true,
      vpcSubnets: subnetSelection,
    });

    const listener = nlb.addListener('TcpListener', {
      port: 80,
      protocol: elbv2.Protocol.TCP,
    });

    listener.addTargets('WsTargets', {
      port: 8080,
      targets: [wsService],
      healthCheck: {
        protocol: elbv2.Protocol.HTTP,
        path: '/health',
      },
    });

    // Auto-scaling
    const scaling = wsService.autoScaleTaskCount({ minCapacity: 3, maxCapacity: 10 });
    scaling.scaleOnCpuUtilization('CpuScaling', { targetUtilizationPercent: 60 });

    // Outputs
    new cdk.CfnOutput(this, 'NlbDns', { value: nlb.loadBalancerDnsName });
    new cdk.CfnOutput(this, 'WebSocketUrl', {
      value: `ws://${nlb.loadBalancerDnsName}/ws`,
    });
    new cdk.CfnOutput(this, 'RedisEndpoint', { value: redisAddr });
  }
}
