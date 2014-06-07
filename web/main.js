var app = angular.module("docker", ['ngRoute']);

app.config(function($routeProvider, $locationProvider) {
    $routeProvider
        .when('/', {
            templateUrl: 'dashboard.html',
            controller: 'DashboardController'
        })

        .when('/containers', {
            templateUrl: 'containers.html',
            controller: 'ContainerController'
        })

        .when('/images', {
            templateUrl: 'images.html',
            controller: 'ImagesController'
        });
    $locationProvider.html5Mode(true);
});

app.controller("MainCtl", function($scope, $route, $routeParams, $location){
    $scope.$route = $route;
    $scope.$location = $location;
    $scope.$routeParams = $routeParams;
    console.log($scope.$location);
    $scope.images = [];
    $scope.containers = [];
    var conn = new WebSocket("ws://localhost:8000/ws");

     conn.onopen = function(e) {
         // make call for data
         $scope.$apply(function(){
             conn.send("{\"command\":\"init\"}");
         });
     }

     conn.onmessage = function(e) {
        $scope.$apply(function() {
            var data = JSON.parse(e.data);
            console.log(data);
            if (data.Type == "full") {
                if (data.Images !== undefined) {
                    $scope.images = data.Images;
                }
                if (data.Containers !== undefined) {
                    $scope.containers = data.Containers;
                }
            } else if (data.Type == "remove") {
                container = $scope.containers.filter(function(c) {
                    return c.ID == data.Containers[0].ID;
                })
                container[0].State.Running = false;
                container[0].NetworkSettings.IPAddress = '';
            } else if (data.Type == "start") {
                // first check if container already in list
                container = $scope.containers.filter(function(c) {
                    return c.ID == data.Containers[0].ID;
                })
                if (container == undefined) {
                    $scope.containers.push(data.Containers[0]);
                } else {
                    container[0].State.Running = true;
                    container[0].NetworkSettings.IPAddress = data.Containers[0].NetworkSettings.IPAddress;
                }
            } else if (data.Type == "destroy") {
                container = $scope.containers.filter(function(c) {
                    return c.ID == data.ID;
                })
                var idx = $scope.containers.indexOf(container[0]);
                if (idx != -1) {
                    $scope.containers.splice(idx, 1);
                }
            }
        })
    };

    $scope.start = function(ID) {
        conn.send("{\"command\":\"start\", \"data\": \""+ ID +"\"}");
    };

    $scope.stop = function(ID) {
        conn.send("{\"command\":\"stop\", \"data\": \""+ ID +"\"}");
    };

    $scope.remove = function(ID) {
        conn.send("{\"command\":\"remove\", \"data\": \""+ ID +"\"}");
    };

    $scope.kill =function(ID) {
        conn.send("{\"command\":\"kill\", \"data\": \""+ ID +"\"}");
    };

    $scope.imageTagsFromID = function (ID) {
        image = $scope.images.filter(function(i) {
            return i.Id == ID;
        })
        return image[0].RepoTags[0];
    };

    $scope.isActive = function (viewLocation) {
        console.log($scope.$location.path());
        return viewLocation === $scope.$location.path();
    };

    $scope.runningContainerFilter = function (item) {
        return (item.State.Running == true);
    };
});


app.controller("DashboardController", function($scope) {

});


app.controller("ContainerController", function($scope) {

});

app.controller("ImagesController", function($scope) {

});
