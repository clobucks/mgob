db = (new Mongo('127.0.0.1:27017')).getDB('test');

rs.initiate({
    _id : "test-set",
    members : [
        {
            _id : 0,
            host : "127.0.0.1:27017"
        }
    ]
});
while(!rs.isMaster().ismaster){ sleep(2000);}

db.getSiblingDB("admin").createUser(
    {
        user: "test",
        pwd: "secrets",
        roles: [ "readWriteAnyDatabase", "userAdminAnyDatabase", "clusterAdmin" ]
    }
);