db = (new Mongo('localhost:27017')).getDB('test');
config = {
    "_id" : "test-set",
    "members" : [
        {
            "_id" : 0,
            "host" : "127.0.0.1:27017"
        }
    ]
};
rs.initiate(config);