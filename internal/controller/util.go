package controller

import "k8s.io/apimachinery/pkg/util/intstr"

func intOrStringFromInt(i int) intstr.IntOrString {
	return intstr.FromInt(i)
}
