#!/bin/bash
NAME=$1
NAMESPACE=$2
PODIDS=$(kubectl get po -o wide -n $NAMESPACE | grep $NAME | awk '{print $1}')
MAXWAITTIMECOUNTER=0

if [ "$PODIDS" != "" ]; then
    for PODID in $PODIDS
    do
        STATE=$(kubectl get po -n $NAMESPACE | grep $PODID | awk '{print $3}')

        KUBECTL=$(kubectl get po -n $NAMESPACE | grep $PODID)
        STATE=$(echo -n $KUBECTL | awk '{print $3}')
        RUNNING=$(echo -n $KUBECTL | awk '{print $2}'|tr '/' ' ')

        if [ "$STATE" == "Running" ]; then
            read READY STARTED <<<$RUNNING
            if [ ! $READY -eq $STARTED ]; then
                STATE=NotReady
            fi
        fi

        echo -n "Wait $NAME on pod $PODID current state $STATE to running"

        while [ "$STATE" != "Running" ]
        do
            KUBECTL=$(kubectl get po -n $NAMESPACE | grep $PODID)
            STATE=$(echo -n $KUBECTL | awk '{print $3}')
            RUNNING=$(echo -n $KUBECTL | awk '{print $2}'|tr '/' ' ')

            if [ $MAXWAITTIMECOUNTER -gt 600 ]; then
                echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
                echo "!!! Max wait time reached !!!"
                echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
                exit -1
            fi

            case "--$STATE" in
                --Error|--CrashLoopBackOff)
                    echo
                    echo "Deployement error $NAME state=$STATE"
                    kubectl --v=8 logs $PODID --namespace=$NAMESPACE -p
                    exit -1
                    ;;
                --ErrImagePull|--ImagePullBackOff)
                    echo
                    echo "Image error $NAME state=$STATE"
                    kubectl --v=8 logs $PODID --namespace=$NAMESPACE -p
                    exit -1
                    ;;
                --CreateContainerConfigError)
                    echo
                    echo "Container error $NAME state=$STATE"
                    kubectl --v=8 logs $PODID --namespace=$NAMESPACE -p
                    exit -1
                    ;;
                --Running)
                    read READY STARTED <<<$RUNNING
                    if [ $READY -eq $STARTED ]; then
                        echo
                        echo "Service $NAME is running"
                        exit 0
                    else
                        STATE=NotReady
                    fi
                    ;;
                --)
                    echo
                    echo "Deployement $NAME not found"
                    exit -1
                    ;;
            esac
	        
            MAXWAITTIMECOUNTER=$((MAXWAITTIMECOUNTER + 1))

            sleep 1

            echo -n "."
        done
    done

    echo
else
    echo "$NAME not found"

    exit -1
fi