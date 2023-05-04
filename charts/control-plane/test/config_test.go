package test

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

func TestDisableSingleReplicaMode(t *testing.T) {
	t.Parallel()
	test := DefaultTest()
	test.Values = `
config:
  kms:
    enabled: true
    key:
      url: base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=
  dataSources:
    postgres:
      enabled: true
      dsn: postgres://localhost:5432/localdb
    prometheus:
      enabled: true
      url: https://localhost:9090
singleReplicaMode:
  enabled: false
`
	expected := DefaultResources(t, test)
	expected.Conf.Value["kms"] = map[string]any{
		"key_url": "base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=",
	}
	expected.Conf.Value["data_sources"] = map[string]any{
		"postgres": map[string]any{
			"dsn": "postgres://localhost:5432/localdb",
		},
		"prometheus": map[string]any{
			"url": "https://localhost:9090",
		},
	}

	expected.Deployment.Value.Spec.Strategy = appsv1.DeploymentStrategy{}

	pts := &expected.Deployment.Value.Spec.Template.Spec
	pts.Volumes = append(pts.Volumes[:2], pts.Volumes[4:]...)
	pts.Volumes[1].PersistentVolumeClaim = nil
	pts.Volumes[1].EmptyDir = &corev1.EmptyDirVolumeSource{}

	ctr := &pts.Containers[0]
	ctr.VolumeMounts = append(ctr.VolumeMounts[:2], ctr.VolumeMounts[4:]...)

	expected.SingleReplicaModeDataPvc.HasValue = false
	expected.SingleReplicaModePostgresPvc.HasValue = false
	expected.SingleReplicaModePrometheusPvc.HasValue = false

	RenderAndCheck(t, test, expected)
}

func TestConfigOptions(t *testing.T) {
	t.Parallel()
	test := DefaultTest()
	test.Values = `
config:
  server:
    url: https://cp.nats.io
    httpPort: 8081
  systems:
    TestContents:
      url: nats://localhost:4222
      systemUserCreds:
        contents: creds
      operatorSigningKey:
        contents: nk
    TestSecretName:
      url: nats://localhost:4222
      systemUserCreds:
        secretName: system-creds
      operatorSigningKey:
        secretName: system-nk
  kms:
    key:
      secretName: key
    rotatedKeys:
    - url: base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=
    - secretName: rotated-key
`
	expected := DefaultResources(t, test)
	expected.Conf.Value["server"] = map[string]any{
		"url":       "https://cp.nats.io",
		"http_addr": ":8081",
	}
	expected.Conf.Value["systems"] = map[string]any{
		"TestContents": map[string]any{
			"url":                       "nats://localhost:4222",
			"system_user_creds_file":    "/etc/syn-cp/contents/TestContents.sys-user.creds",
			"operator_signing_key_file": "/etc/syn-cp/contents/TestContents.operator-sk.nk",
		},
		"TestSecretName": map[string]any{
			"url":                       "nats://localhost:4222",
			"system_user_creds_file":    "/etc/syn-cp/systems/TestSecretName/sys-user-creds/sys-user.creds",
			"operator_signing_key_file": "/etc/syn-cp/systems/TestSecretName/operator-sk/operator-sk.nk",
		},
	}
	expected.Conf.Value["kms"] = map[string]any{
		"key_url": "file:///etc/syn-cp/kms/key.enc",
		"rotated_key_urls": []any{
			"base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=",
			"file:///etc/syn-cp/kms/rotated-key-1/key.enc",
		},
	}

	expected.ContentsSecret.HasValue = true
	expected.ContentsSecret.Value.StringData = map[string]string{
		"TestContents.sys-user.creds": "creds",
		"TestContents.operator-sk.nk": "nk",
	}

	pts := &expected.Deployment.Value.Spec.Template.Spec
	pts.Volumes = append(pts.Volumes, corev1.Volume{
		Name: "contents",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "control-plane-contents",
			},
		},
	})

	ctr := &pts.Containers[0]
	ctr.VolumeMounts = append(ctr.VolumeMounts, corev1.VolumeMount{
		MountPath: "/etc/syn-cp/contents",
		Name:      "contents",
	})

	ctr.Ports[0].ContainerPort = 8081

	RenderAndCheck(t, test, expected)
}

//func TestConfigMergePatch(t *testing.T) {
//	t.Parallel()
//	test := DefaultTest()
//	test.Values = `
//config:
//  merge:
//    ping_interval: 5m
//  patch: [{op: add, path: /ping_max, value: 3}]
//  cluster:
//    enabled: true
//    merge:
//      no_advertise: false
//    patch: [{op: add, path: /advertise, value: "demo.nats.io:6222"}]
//  jetstream:
//    enabled: true
//    merge:
//      max_outstanding_catchup: "<< 64MB >>"
//    patch: [{op: add, path: /max_file_store, value: "<< 1GB >>"}]
//    fileStore:
//      pvc:
//        merge:
//          spec:
//            storageClassName: gp3
//        patch: [{op: add, path: /spec/accessModes/-, value: ReadWriteMany}]
//  leafnode:
//    enabled: true
//    merge:
//      no_advertise: false
//    patch: [{op: add, path: /advertise, value: "demo.nats.io:7422"}]
//  websocket:
//    enabled: true
//    merge:
//      compression: false
//    patch: [{op: add, path: /same_origin, value: true}]
//  mqtt:
//    enabled: true
//    merge:
//      ack_wait: 1m
//    patch: [{op: add, path: /max_ack_pending, value: 100}]
//  gateway:
//    enabled: true
//    merge:
//      gateways:
//      - name: nats
//        url: nats://demo.nats.io:7222
//    patch: [{op: add, path: /advertise, value: "demo.nats.io:7222"}]
//  resolver:
//    enabled: true
//    merge:
//      type: full
//    patch: [{op: add, path: /allow_delete, value: true}]
//    pvc:
//      merge:
//        spec:
//          storageClassName: gp3
//      patch: [{op: add, path: /spec/accessModes/-, value: ReadWriteMany}]
//`
//	expected := DefaultResources(t, test)
//	expected.Conf.Value["ping_interval"] = "5m"
//	expected.Conf.Value["ping_max"] = int64(3)
//	expected.Conf.Value["cluster"] = map[string]any{
//		"name":         "nats",
//		"no_advertise": false,
//		"advertise":    "demo.nats.io:6222",
//		"port":         int64(6222),
//		"routes": []any{
//			"nats://nats-0.nats-headless:6222",
//			"nats://nats-1.nats-headless:6222",
//			"nats://nats-2.nats-headless:6222",
//		},
//	}
//	expected.Conf.Value["jetstream"] = map[string]any{
//		"max_memory_store":        int64(0),
//		"store_dir":               "/data",
//		"max_file_store":          int64(1073741824),
//		"max_outstanding_catchup": int64(67108864),
//	}
//	expected.Conf.Value["leafnode"] = map[string]any{
//		"port":         int64(7422),
//		"no_advertise": false,
//		"advertise":    "demo.nats.io:7422",
//	}
//	expected.Conf.Value["websocket"] = map[string]any{
//		"port":        int64(8080),
//		"compression": false,
//		"no_tls":      true,
//		"same_origin": true,
//	}
//	expected.Conf.Value["mqtt"] = map[string]any{
//		"port":            int64(1883),
//		"ack_wait":        "1m",
//		"max_ack_pending": int64(100),
//	}
//	expected.Conf.Value["gateway"] = map[string]any{
//		"port":      int64(7222),
//		"name":      "nats",
//		"advertise": "demo.nats.io:7222",
//		"gateways": []any{
//			map[string]any{
//				"name": "nats",
//				"url":  "nats://demo.nats.io:7222",
//			},
//		},
//	}
//	expected.Conf.Value["resolver"] = map[string]any{
//		"dir":          "/data/resolver",
//		"type":         "full",
//		"allow_delete": true,
//	}
//
//	replicas3 := int32(3)
//	expected.StatefulSet.Value.Spec.Replicas = &replicas3
//
//	vm := expected.StatefulSet.Value.Spec.Template.Spec.Containers[0].VolumeMounts
//	expected.StatefulSet.Value.Spec.Template.Spec.Containers[0].VolumeMounts = append(vm, corev1.VolumeMount{
//		MountPath: "/data/jetstream",
//		Name:      test.FullName + "-js",
//	}, corev1.VolumeMount{
//		MountPath: "/data/resolver",
//		Name:      test.FullName + "-resolver",
//	})
//
//	resource1Gi, _ := resource.ParseQuantity("1Gi")
//	resource10Gi, _ := resource.ParseQuantity("10Gi")
//	storageClassGp3 := "gp3"
//	expected.StatefulSet.Value.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
//		{
//			ObjectMeta: corev1.ObjectMeta{
//				Name: test.FullName + "-js",
//			},
//			Spec: corev1.PersistentVolumeClaimSpec{
//				AccessModes: []corev1.PersistentVolumeAccessMode{
//					"ReadWriteOnce",
//					"ReadWriteMany",
//				},
//				Resources: corev1.ResourceRequirements{
//					Requests: corev1.ResourceList{
//						"storage": resource10Gi,
//					},
//				},
//				StorageClassName: &storageClassGp3,
//			},
//		},
//		{
//			ObjectMeta: corev1.ObjectMeta{
//				Name: test.FullName + "-resolver",
//			},
//			Spec: corev1.PersistentVolumeClaimSpec{
//				AccessModes: []corev1.PersistentVolumeAccessMode{
//					"ReadWriteOnce",
//					"ReadWriteMany",
//				},
//				Resources: corev1.ResourceRequirements{
//					Requests: corev1.ResourceList{
//						"storage": resource1Gi,
//					},
//				},
//				StorageClassName: &storageClassGp3,
//			},
//		},
//	}
//
//	expected.StatefulSet.Value.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
//		{
//			Name:          "nats",
//			ContainerPort: 4222,
//		},
//		{
//			Name:          "leafnode",
//			ContainerPort: 7422,
//		},
//		{
//			Name:          "websocket",
//			ContainerPort: 8080,
//		},
//		{
//			Name:          "mqtt",
//			ContainerPort: 1883,
//		},
//		{
//			Name:          "cluster",
//			ContainerPort: 6222,
//		},
//		{
//			Name:          "gateway",
//			ContainerPort: 7222,
//		},
//		{
//			Name:          "monitor",
//			ContainerPort: 8222,
//		},
//	}
//
//	expected.HeadlessService.Value.Spec.Ports = []corev1.ServicePort{
//		{
//			Name:       "nats",
//			Port:       4222,
//			TargetPort: intstr.FromString("nats"),
//		},
//		{
//			Name:       "leafnode",
//			Port:       7422,
//			TargetPort: intstr.FromString("leafnode"),
//		},
//		{
//			Name:       "websocket",
//			Port:       8080,
//			TargetPort: intstr.FromString("websocket"),
//		},
//		{
//			Name:       "mqtt",
//			Port:       1883,
//			TargetPort: intstr.FromString("mqtt"),
//		},
//		{
//			Name:       "cluster",
//			Port:       6222,
//			TargetPort: intstr.FromString("cluster"),
//		},
//		{
//			Name:       "gateway",
//			Port:       7222,
//			TargetPort: intstr.FromString("gateway"),
//		},
//		{
//			Name:       "monitor",
//			Port:       8222,
//			TargetPort: intstr.FromString("monitor"),
//		},
//	}
//
//	expected.Service.Value.Spec.Ports = []corev1.ServicePort{
//		{
//			Name:       "nats",
//			Port:       4222,
//			TargetPort: intstr.FromString("nats"),
//		},
//		{
//			Name:       "leafnode",
//			Port:       7422,
//			TargetPort: intstr.FromString("leafnode"),
//		},
//		{
//			Name:       "websocket",
//			Port:       8080,
//			TargetPort: intstr.FromString("websocket"),
//		},
//		{
//			Name:       "mqtt",
//			Port:       1883,
//			TargetPort: intstr.FromString("mqtt"),
//		},
//	}
//
//	RenderAndCheck(t, test, expected)
//}
//
//func TestConfigTls(t *testing.T) {
//	t.Parallel()
//	test := DefaultTest()
//	test.Values = `
//config:
//  cluster:
//    enabled: true
//    tls:
//      enabled: true
//      secretName: cluster-tls
//  nats:
//    tls:
//      enabled: true
//      secretName: nats-tls
//      ca: tls.ca
//      merge:
//        verify_cert_and_check_known_urls: true
//      patch: [{op: add, path: /verify_and_map, value: true}]
//  leafnode:
//    enabled: true
//    tls:
//      enabled: true
//      secretName: leafnode-tls
//  websocket:
//    enabled: true
//    tls:
//      enabled: true
//      secretName: websocket-tls
//  mqtt:
//    enabled: true
//    tls:
//      enabled: true
//      secretName: mqtt-tls
//  gateway:
//    enabled: true
//    tls:
//      enabled: true
//      secretName: gateway-tls
//`
//	expected := DefaultResources(t, test)
//	expected.Conf.Value["cluster"] = map[string]any{
//		"name":         "nats",
//		"no_advertise": true,
//		"port":         int64(6222),
//		"routes": []any{
//			"tls://nats-0.nats-headless:6222",
//			"tls://nats-1.nats-headless:6222",
//			"tls://nats-2.nats-headless:6222",
//		},
//	}
//	expected.Conf.Value["leafnode"] = map[string]any{
//		"port":         int64(7422),
//		"no_advertise": true,
//	}
//	expected.Conf.Value["websocket"] = map[string]any{
//		"port":        int64(8080),
//		"compression": true,
//	}
//	expected.Conf.Value["mqtt"] = map[string]any{
//		"port": int64(1883),
//	}
//	expected.Conf.Value["gateway"] = map[string]any{
//		"port": int64(7222),
//		"name": "nats",
//	}
//
//	replicas3 := int32(3)
//	expected.StatefulSet.Value.Spec.Replicas = &replicas3
//
//	volumes := expected.StatefulSet.Value.Spec.Template.Spec.Volumes
//	natsVm := expected.StatefulSet.Value.Spec.Template.Spec.Containers[0].VolumeMounts
//	reloaderVm := expected.StatefulSet.Value.Spec.Template.Spec.Containers[1].VolumeMounts
//	for _, protocol := range []string{"nats", "leafnode", "websocket", "mqtt", "cluster", "gateway"} {
//		tls := map[string]any{
//			"cert_file": "/etc/nats-certs/" + protocol + "/tls.crt",
//			"key_file":  "/etc/nats-certs/" + protocol + "/tls.key",
//		}
//		if protocol == "nats" {
//			tls["ca_file"] = "/etc/nats-certs/" + protocol + "/tls.ca"
//			tls["verify"] = true
//			tls["verify_cert_and_check_known_urls"] = true
//			tls["verify_and_map"] = true
//			expected.Conf.Value["tls"] = tls
//		} else {
//			expected.Conf.Value[protocol].(map[string]any)["tls"] = tls
//		}
//
//		volumes = append(volumes, corev1.Volume{
//			Name: protocol + "-tls",
//			VolumeSource: corev1.VolumeSource{
//				Secret: &corev1.SecretVolumeSource{
//					SecretName: protocol + "-tls",
//				},
//			},
//		})
//
//		natsVm = append(natsVm, corev1.VolumeMount{
//			MountPath: "/etc/nats-certs/" + protocol,
//			Name:      protocol + "-tls",
//		})
//
//		reloaderVm = append(reloaderVm, corev1.VolumeMount{
//			MountPath: "/etc/nats-certs/" + protocol,
//			Name:      protocol + "-tls",
//		})
//	}
//
//	expected.StatefulSet.Value.Spec.Template.Spec.Volumes = volumes
//	expected.StatefulSet.Value.Spec.Template.Spec.Containers[0].VolumeMounts = natsVm
//	expected.StatefulSet.Value.Spec.Template.Spec.Containers[1].VolumeMounts = reloaderVm
//
//	// reloader certs are alphabetized
//	reloaderArgs := expected.StatefulSet.Value.Spec.Template.Spec.Containers[1].Args
//	for _, protocol := range []string{"cluster", "gateway", "leafnode", "mqtt", "nats", "websocket"} {
//		if protocol == "nats" {
//			reloaderArgs = append(reloaderArgs, "-config", "/etc/nats-certs/"+protocol+"/tls.ca")
//		}
//		reloaderArgs = append(reloaderArgs, "-config", "/etc/nats-certs/"+protocol+"/tls.crt", "-config", "/etc/nats-certs/"+protocol+"/tls.key")
//	}
//
//	expected.StatefulSet.Value.Spec.Template.Spec.Containers[1].Args = reloaderArgs
//
//	expected.StatefulSet.Value.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
//		{
//			Name:          "nats",
//			ContainerPort: 4222,
//		},
//		{
//			Name:          "leafnode",
//			ContainerPort: 7422,
//		},
//		{
//			Name:          "websocket",
//			ContainerPort: 8080,
//		},
//		{
//			Name:          "mqtt",
//			ContainerPort: 1883,
//		},
//		{
//			Name:          "cluster",
//			ContainerPort: 6222,
//		},
//		{
//			Name:          "gateway",
//			ContainerPort: 7222,
//		},
//		{
//			Name:          "monitor",
//			ContainerPort: 8222,
//		},
//	}
//
//	expected.HeadlessService.Value.Spec.Ports = []corev1.ServicePort{
//		{
//			Name:       "nats",
//			Port:       4222,
//			TargetPort: intstr.FromString("nats"),
//		},
//		{
//			Name:       "leafnode",
//			Port:       7422,
//			TargetPort: intstr.FromString("leafnode"),
//		},
//		{
//			Name:       "websocket",
//			Port:       8080,
//			TargetPort: intstr.FromString("websocket"),
//		},
//		{
//			Name:       "mqtt",
//			Port:       1883,
//			TargetPort: intstr.FromString("mqtt"),
//		},
//		{
//			Name:       "cluster",
//			Port:       6222,
//			TargetPort: intstr.FromString("cluster"),
//		},
//		{
//			Name:       "gateway",
//			Port:       7222,
//			TargetPort: intstr.FromString("gateway"),
//		},
//		{
//			Name:       "monitor",
//			Port:       8222,
//			TargetPort: intstr.FromString("monitor"),
//		},
//	}
//
//	expected.Service.Value.Spec.Ports = []corev1.ServicePort{
//		{
//			Name:       "nats",
//			Port:       4222,
//			TargetPort: intstr.FromString("nats"),
//		},
//		{
//			Name:       "leafnode",
//			Port:       7422,
//			TargetPort: intstr.FromString("leafnode"),
//		},
//		{
//			Name:       "websocket",
//			Port:       8080,
//			TargetPort: intstr.FromString("websocket"),
//		},
//		{
//			Name:       "mqtt",
//			Port:       1883,
//			TargetPort: intstr.FromString("mqtt"),
//		},
//	}
//
//	RenderAndCheck(t, test, expected)
//}
//
//func TestConfigInclude(t *testing.T) {
//	t.Parallel()
//	test := DefaultTest()
//	test.Values = `
//config:
//  jetstream:
//    enabled: true
//    merge:
//      000$include: "js.conf"
//  merge:
//    $include: "my-config.conf"
//    zzz$include: "my-config-last.conf"
//configMap:
//  merge:
//    data:
//      js.conf: |
//        max_file_store:  1GB
//        max_outstanding_catchup: 64MB
//      my-config.conf: |
//        ping_interval: "5m"
//      my-config-last.conf: |
//        ping_max: 3
//`
//	expected := DefaultResources(t, test)
//	expected.Conf.Value["ping_interval"] = "5m"
//	expected.Conf.Value["ping_max"] = int64(3)
//	expected.Conf.Value["jetstream"] = map[string]any{
//		"max_file_store":          int64(1073741824),
//		"max_memory_store":        int64(0),
//		"max_outstanding_catchup": int64(67108864),
//		"store_dir":               "/data",
//	}
//
//	expected.ConfigMap.Value.Data = map[string]string{
//		"js.conf": `max_file_store:  1GB
//max_outstanding_catchup: 64MB
//`,
//		"my-config.conf": `ping_interval: "5m"
//`,
//		"my-config-last.conf": `ping_max: 3
//`,
//	}
//
//	reloaderArgs := expected.StatefulSet.Value.Spec.Template.Spec.Containers[1].Args
//	reloaderArgs = append(reloaderArgs, "-config", "/etc/nats-config/my-config.conf")
//	reloaderArgs = append(reloaderArgs, "-config", "/etc/nats-config/js.conf")
//	reloaderArgs = append(reloaderArgs, "-config", "/etc/nats-config/my-config-last.conf")
//	expected.StatefulSet.Value.Spec.Template.Spec.Containers[1].Args = reloaderArgs
//
//	vm := expected.StatefulSet.Value.Spec.Template.Spec.Containers[0].VolumeMounts
//	expected.StatefulSet.Value.Spec.Template.Spec.Containers[0].VolumeMounts = append(vm, corev1.VolumeMount{
//		MountPath: "/data/jetstream",
//		Name:      test.FullName + "-js",
//	})
//
//	resource10Gi, _ := resource.ParseQuantity("10Gi")
//	expected.StatefulSet.Value.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
//		{
//			ObjectMeta: corev1.ObjectMeta{
//				Name: test.FullName + "-js",
//			},
//			Spec: corev1.PersistentVolumeClaimSpec{
//				AccessModes: []corev1.PersistentVolumeAccessMode{
//					"ReadWriteOnce",
//				},
//				Resources: corev1.ResourceRequirements{
//					Requests: corev1.ResourceList{
//						"storage": resource10Gi,
//					},
//				},
//			},
//		},
//	}
//
//	RenderAndCheck(t, test, expected)
//}
//
//func TestExtraResources(t *testing.T) {
//	t.Parallel()
//	test := DefaultTest()
//	test.Values = `
//extraResources:
//- apiVersion: corev1
//  kind: Service
//  metadata:
//    name:
//      $tplYaml: >
//        {{ include "scp.fullname" $ }}-extra
//    labels:
//      $tplYaml: |
//        {{ include "scp.labels" $ }}
//  spec:
//    selector:
//      labels:
//        $tplYamlSpread: |
//          {{ include "scp.selectorLabels" $ | nindent 4 }}
//    ports:
//    - $tplYamlSpread: |
//        - name: gateway
//          port: 7222
//          targetPort: gateway
//- $tplYaml: |
//    apiVersion: corev1
//    kind: ConfigMap
//    metadata:
//      name: {{ include "scp.fullname" $ }}-extra
//      labels:
//        {{- include "scp.labels" $ | nindent 4 }}
//    data:
//      foo: bar
//`
//
//	expected := DefaultResources(t, test)
//
//	expected.ExtraConfigMap.HasValue = true
//	expected.ExtraConfigMap.Value.Data = map[string]string{
//		"foo": "bar",
//	}
//
//	expected.ExtraService.HasValue = true
//	expected.ExtraService.Value.Spec.Ports = []corev1.ServicePort{
//		{
//			Name:       "gateway",
//			Port:       7222,
//			TargetPort: intstr.FromString("gateway"),
//		},
//	}
//
//	RenderAndCheck(t, test, expected)
//}
