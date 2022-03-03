package v1

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type OwnStatefulSet struct {
	Spec appsv1.StatefulSetSpec
}

var rsName string

func (ownStatefulSet *OwnStatefulSet) MakeOwnResource(instance *Mymongodb, logger logr.Logger,
	scheme *runtime.Scheme) (interface{}, error) {

	// new a StatefulSet object
	sts := &appsv1.StatefulSet{
		// metadata field inherited from owner Unit
		ObjectMeta: metav1.ObjectMeta{Name: instance.Name, Namespace: instance.Namespace, Labels: instance.Labels},
		Spec:       ownStatefulSet.Spec,
	}

	// add some customize envs, ignore this step if you don't need it
	customizeEnvs := []v1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "metadata.name",
				},
			},
		},
		{
			Name:  "APPNAME",
			Value: instance.Name,
		},
	}

	var specEnvs []v1.EnvVar
	templateEnvs := sts.Spec.Template.Spec.Containers[0].Env
	for index := range templateEnvs {
		if templateEnvs[index].Name != "POD_NAME" && templateEnvs[index].Name != "APPNAME" {
			specEnvs = append(specEnvs, templateEnvs[index])
		}
		if templateEnvs[index].Name == "REPLSET_NAME" {
			rsName = templateEnvs[index].Value
		}
	}

	sts.Spec.Template.Spec.Containers[0].Env = append(specEnvs, customizeEnvs...)

	// add ControllerReference for sts，the owner is Unit object
	if err := controllerutil.SetControllerReference(instance, sts, scheme); err != nil {
		msg := fmt.Sprintf("set controllerReference for StatefulSet %s/%s failed", instance.Namespace, instance.Name)
		logger.Error(err, msg)
		return nil, err
	}

	return sts, nil
}

// Check if the StatefulSet already exists
func (ownStatefulSet *OwnStatefulSet) OwnResourceExist(instance *Mymongodb, client client.Client,
	logger logr.Logger) (bool, interface{}, error) {

	found := &appsv1.StatefulSet{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		msg := fmt.Sprintf("StatefulSet %s/%s found, but with error", instance.Namespace, instance.Name)
		logger.Error(err, msg)
		return true, found, err
	}
	return true, found, nil
}

func (ownStatefulSet *OwnStatefulSet) UpdateOwnResourceStatus(instance *Mymongodb, client client.Client,
	logger logr.Logger) (*Mymongodb, error) {

	found := &appsv1.StatefulSet{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
	if err != nil {
		return instance, err
	}
	instance.Status.BaseStatefulSet = found.Status
	instance.Status.LastUpdateTime = metav1.Now()
	return instance, nil

	//if err := client.Status().Update(context.Background(), instance); err != nil {
	//	logger.Error(err, "unable to update Unit StatefulSet status")
	//	return instance, err
	//}

}

// apply this own resource, create or update
func (ownStatefulSet *OwnStatefulSet) ApplyOwnResource(instance *Mymongodb, client client.Client,
	logger logr.Logger, scheme *runtime.Scheme) error {

	// assert if StatefulSet exist
	exist, found, err := ownStatefulSet.OwnResourceExist(instance, client, logger)
	if err != nil {
		return err
	}

	// make StatefulSet object
	sts, err := ownStatefulSet.MakeOwnResource(instance, logger, scheme)
	if err != nil {
		return err
	}
	newStatefulSet := sts.(*appsv1.StatefulSet)

	// apply the StatefulSet object just make
	if !exist {
		// if StatefulSet not exist，then create it
		//labelMap := make(map[string]string, 1)
		//labelMap["app"] = newStatefulSet.Name
		//newStatefulSet.Spec.Selector = &metav1.LabelSelector{MatchLabels: labelMap}
		//newStatefulSet.Spec.Template.Labels = labelMap
		msg := fmt.Sprintf("StatefulSet %s/%s not found, create it!", newStatefulSet.Namespace, newStatefulSet.Name)
		logger.Info(msg)
		if err := client.Create(context.TODO(), newStatefulSet); err != nil {
			logger.Info("===status create fail =====", err)

			return err
		}
		// 组建mongodb集群 逻辑
		go buildMongoCluster(newStatefulSet, logger, true)

		return nil

	} else {
		foundStatefulSet := found.(*appsv1.StatefulSet)
		// if StatefulSet exist with change，then try to update it
		if !reflect.DeepEqual(newStatefulSet.Spec, foundStatefulSet.Spec) {
			msg := fmt.Sprintf("Updating StatefulSet %s/%s", newStatefulSet.Namespace, newStatefulSet.Name)
			logger.Info(msg)

			err1 := client.Update(context.TODO(), newStatefulSet)
			// 组建mongodb集群 逻辑
			if *newStatefulSet.Spec.Replicas != *foundStatefulSet.Spec.Replicas {
				go buildMongoCluster(foundStatefulSet, logger, false)
			}

			return err1
		}

		return nil
	}
}

func buildMongoCluster(instance *appsv1.StatefulSet, logger logr.Logger, initFlag bool) {
	logger.Info("============buildMongoCluster====================================")
	time.Sleep(time.Second * 10)
	mongoAddr := []string{}
	Replicas := int(*instance.Spec.Replicas)
	members := bson.A{}
	for v := 0; v < Replicas; v++ {
		addr := instance.Name + "-" + strconv.Itoa(v) + "." + instance.Name + "." + instance.Namespace + ".svc.cluster.local:" + strconv.Itoa(int(instance.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort))
		// _, err := net.Dial("tcp", addr)
		// if err != nil {
		// 	logger.Error(err, "mongoAddr not ping")
		// }
		members = append(members, bson.D{{Key: "_id", Value: v}, {Key: "host", Value: addr}})
		mongoAddr = append(mongoAddr, addr)
	}
	cfg := bson.D{{Key: "_id", Value: rsName}, {Key: "members", Value: members}}
	for v := 0; v < Replicas; v++ {
		clientOptions := options.Client().ApplyURI(fmt.Sprintf("mongodb://%s/admin", mongoAddr[v]))
		clientOptions.SetAuth(options.Credential{
			Username: "monitor",
			Password: "monitor",
		})
		clientOptions.SetDirect(true)

		// 连接到MongoDB
		mongoC, err := mongo.Connect(context.TODO(), clientOptions)
		if err != nil {
			logger.Error(err, "Failed to create mongodb connect")
			continue
		}

		// 检查连接
		err = mongoC.Ping(context.TODO(), nil)
		if err != nil {
			logger.Error(err, "Failed to connect mongodb ")
			continue
		}
		if initFlag {
			err = mongoC.Database("admin").RunCommand(context.TODO(), bson.D{
				{Key: "replSetInitiate", Value: cfg},
			}).Err()
			if err != nil {
				logger.Info("Mongodb Node: ", mongoAddr[v], " didn't set up a cluster!")
				continue
			}
		} else {
			err = mongoC.Database("admin").RunCommand(context.TODO(), bson.D{
				{Key: "replSetReconfig", Value: cfg},
				{Key: "force", Value: true},
			}).Err()
			if err != nil {
				logger.Info("Mongodb Node: ", mongoAddr[v], " didn't set up a cluster!")
				continue
			}

		}

		return
	}

}
