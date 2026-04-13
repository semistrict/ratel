# Copyright 2026 The Ratel Authors
# Licensed under the Apache 2.0 license found in the LICENSE file or at:
#     https://opensource.org/licenses/Apache-2.0
#
# This is a Go-annotated copy of workerd's actor-storage.capnp.
# The interface IDs must match the C++ side exactly for wire compatibility.

@0xb200a391b94343f1;

using Go = import "/go.capnp";
$Go.package("actorstorage");
$Go.import("github.com/semistrict/ratel/pkg/server/actorstorage");

interface ActorStorage @0xd7759d7fc87c08e4 {
  struct KeyValue {
    key @0 :Data;
    value @1 :Data;
  }

  struct KeyRename {
    oldKey @0 :Data;
    newKey @1 :Data;
  }

  getStage @0 (stableId :Text) -> (stage :Stage);

  interface Operations @0xb512f2ce1f544439 {
    get @0 (key :Data) -> (value :Data);
    list @3 (start :Data, end :Data, limit :Int32, reverse :Bool, stream :ListStream, prefix :Data);
    put @1 (entries :List(KeyValue));
    delete @2 (keys :List(Data)) -> (numDeleted :Int32);

    getMultiple @4 (keys :List(Data), stream :ListStream);
    deleteAll @5 () -> (numDeleted :Int32);

    rename @9 (entries :List(KeyRename)) -> (renamed :List(Data));

    getAlarm @6 () -> (scheduledTimeMs :Int64);
    setAlarm @7 (scheduledTimeMs :Int64);
    deleteAlarm @8 (timeToDeleteMs :Int64) -> (deleted :Bool);
  }

  struct DbSettings {
    enum Priority {
      default @0;
      low @1;
    }
    priority @0 :Priority;
    asOfTimeMs @1 :Int64;
  }

  interface Stage @0xdc35f52864c57550 extends(Operations) {
    txn @0 (settings :DbSettings) -> (transaction :Transaction);

    interface Transaction extends(Operations) {
      commit @0 ();
      rollback @1 ();
    }
  }

  interface ListStream {
    values @0 (list :List(KeyValue)) -> stream;
    end @1 ();
  }

  const maxKeys :UInt32 = 128;
  const renameLimit :UInt32 = 1000;
}
